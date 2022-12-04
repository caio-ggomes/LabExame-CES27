package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

// Variáveis globais interessantes para o processo

var msgCounter int                        // Contador de mensagens enviadas
var err string                            // String de erro em algum procedimento
var myPort string                         // Porta do meu servidor
var CliConn map[*net.UDPAddr]*net.UDPConn // Dicionário com conexões para os servidores dos outros processos
var CoordAddr *net.UDPAddr                // Endereço da conexão com o coordenador
var CoordConn *net.UDPConn                // Conexão para o servidor do coordenador

var myId int               // Identidade deste processo
var requested bool         // Requisitou token
var coordinatorId int      // Identidade do processo coordenador
var myToken bool           // Coordenador tem ou não o token atualmente
var myQueue []*net.UDPAddr // Fila de endereços dos processos que pediram Request ao coordenador

var mutexTokenQueue sync.Mutex // Mutex para garantir correta manipulação da variável Fila
var mutexRequest sync.Mutex    // Mutex para garantir correta manipulação da variável request

var ServerConn *net.UDPConn // Conexão do servidor deste processo(onde são recebidas mensagens dos outros processos)

// Rotina para ler a entrada do processo e capturar os caracteres
func readInput(ch chan string) {
	reader := bufio.NewReader(os.Stdin)
	for {
		text, _, _ := reader.ReadLine()
		ch <- string(text)
	}
}

// Método para checar se houve erro e apresentá-lo
func CheckError(err error) {
	if err != nil {
		fmt.Println("Erro: ", err)
		os.Exit(0)
	}
}

// Rotina para acessar o recurso compartilhado
func accessSharedResource() {
	mutexRequest.Lock()
	fmt.Println("Entrei na CS")
	// Tempo artificial de acesso do recurso compartilhado
	time.Sleep(time.Second * 5)
	requested = false
	ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+":10001")
	CheckError(err)
	// Enviar mensagem com ID e TIMESTAMP, além de um texto básico
	_, err = ServerConn.WriteToUDP([]byte("ID: "+strconv.Itoa(myId)+"\nACESSEI A CS\n"), ServerAddr)
	CheckError(err)
	fmt.Println("Saí da CS")
	go doClientJob(CoordAddr, "RELEASE TOKEN")
	mutexRequest.Unlock()
}

func doServerJob() {

	buf := make([]byte, 1024)
	// Ler da conexão UDP a mensagem no buffer e tomar as devidas medidas em caso de receber REQUEST ou RELEASE

	// Server job do coordenador
	if myId == coordinatorId {
		for {
			n, addr, err := ServerConn.ReadFromUDP(buf)
			CheckError(err)
			msg := string(buf[0:n])
			fmt.Println("Received "+msg+" from ", addr)
			if msg[0:13] == "REQUEST TOKEN" {
				mutexTokenQueue.Lock()
				// Se receber request e tiver o token, garantir ele para o requisitante
				if myToken {
					myToken = false
					go doClientJob(addr, "GRANT TOKEN")
				} else {
					// Caso nao possua o token atualmente, enfileirar
					myQueue = append(myQueue, addr)
				}
				mutexTokenQueue.Unlock()
			} else if msg[0:13] == "RELEASE TOKEN" {
				// Caso receba release, fica detentor do token e, se tiver processos na fila, garantir o token ao primeiro elemento desta
				myToken = true
				if len(myQueue) > 0 {
					mutexTokenQueue.Lock()
					myToken = false
					go doClientJob(myQueue[0], "GRANT TOKEN")
					myQueue = myQueue[1:]
					mutexTokenQueue.Unlock()
				}
			}
		}
	} else {
		// Server job do processo
		for {
			n, addr, err := ServerConn.ReadFromUDP(buf)
			CheckError(err)
			msg := string(buf[0:n])
			fmt.Println("Received "+msg+" from ", addr)
			// Se recebeu grant, acessar o recurso compartilhado
			if msg[0:11] == "GRANT TOKEN" {
				go accessSharedResource()
			}
		}
	}
}

// Rotina para enviar mensagens para outros processos
func doClientJob(otherProcessAddress *net.UDPAddr, msg string) {
	msgCounter++
	buf := make([]byte, 1024)
	buf = []byte(msg)
	_, err := ServerConn.WriteToUDP(buf, otherProcessAddress)
	CheckError(err)
}

// Método para iniciar as conexões entre todos os processos
func initConnections() {

	CliConn = make(map[*net.UDPAddr]*net.UDPConn)

	// Conexão para escutar

	ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+myPort)
	CheckError(err)
	ServerConn, err = net.ListenUDP("udp", ServerAddr)
	CheckError(err)

	// Se não for coordenador, estabelecer uma conexão com este para escrever
	if myId != coordinatorId {
		coordinatorPort := os.Args[5]
		ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+coordinatorPort)
		CoordAddr = ServerAddr
		CheckError(err)
		Conn, err := net.DialUDP("udp", nil, CoordAddr)
		CheckError(err)
		CoordConn = Conn
		CliConn[CoordAddr] = CoordConn
	}
}

// A entrada para o coordenador será da forma ./Process c {myId} {myPort}
// Já para o processo, a entrada será ./Process p {myId} {myPort} {Coordinator_id} {Coordinator_port}
func main() {

	// Inicialização das variáveis
	msgCounter = 0
	id, err := strconv.Atoi(os.Args[2])
	CheckError(err)
	myId = id
	myPort = os.Args[3]
	myQueue = make([]*net.UDPAddr, 0)

	if os.Args[1] == "c" {
		coordinatorId = myId
		myToken = true
	} else {
		id, err := strconv.Atoi(os.Args[4])
		CheckError(err)
		coordinatorId = id
	}

	// Iniciar conexões
	initConnections()

	// O fechamento de conexões ocorrerá quando a main retornar
	defer ServerConn.Close()
	for _, conn := range CliConn {
		defer conn.Close()
	}

	ch := make(chan string) // Canal que registra caracteres digitados
	go readInput(ch)        // Chamar rotina que ”escuta” o teclado

	// Escutar outros processos
	go doServerJob()
	if myId != coordinatorId {
		for {
			// Verificar (de forma não bloqueante) se tem algo no stdin (input do terminal)
			select {
			case x, valid := <-ch:
				if valid {
					fmt.Printf("Recebi do teclado: %s \n", x)
					// Se a mensagem for "x", então pediu acesso à CS
					if x == "x" {
						// Pedido indevido
						if requested {
							fmt.Println("x ignorado")
						} else {
							requested = true
							go doClientJob(CoordAddr, "REQUEST TOKEN")
						}
					}
					if x == "y" {
						fmt.Println(msgCounter)
					}
				} else {
					fmt.Println("Canal fechado!")
				}
			default:
				// Fazer nada!
				// Mas não fica bloqueado esperando o teclado
				time.Sleep(time.Millisecond * 400)
			}

			// Espera um pouco
			time.Sleep(time.Millisecond * 400)
		}
	} else {
		for {
			select {
			case x, valid := <-ch:
				if valid {
					fmt.Printf("Recebi do teclado: %s \n", x)

					if x == "y" {
						fmt.Println(msgCounter)
					}
				} else {
					fmt.Println("Canal fechado!")
				}
			default:
				// Fazer nada!
				// Mas não fica bloqueado esperando o teclado
				time.Sleep(time.Millisecond * 400)
			}
			// Espera um pouco
			time.Sleep(time.Millisecond * 400)
		}
	}

}
