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

var msgCounter int // Contador de mensagens enviadas
var err string     // String de erro em caso de falha em algum procedimento
var myPort string  // Porta do meu servidor

var myId int // Identidade deste processo

var leftConn *net.UDPConn   // Conexão com filho esquerdo
var rightConn *net.UDPConn  // Conexão com filho direito
var parentConn *net.UDPConn // Conexão com pai

var addrMap map[int]*net.UDPAddr // Dicionário com endereço dos vizinhos baseado no ID
var revPortMap map[int]int       // Dicionário com ID dos vizinhos baseado no endereço

var forwardRequest bool // Encaminhou request para alguém desde a última vez que recebeu token
var holder int          // Vizinho que está no caminho para o detentor do token, ou o próprio processo, em caso de possuir o token
var myQueue []int       // Fila de processos que pediram Request

var mutexTokenQueue sync.Mutex // Mutex para garantir correta manipulação da variável Fila

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
	fmt.Println("Entrei na CS")
	ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+":10001")
	CheckError(err)
	// Enviar mensagem com ID e TIMESTAMP, além de um texto básico
	_, err = ServerConn.WriteToUDP([]byte("ID: "+strconv.Itoa(myId)+"\nACESSEI A CS\n"), ServerAddr)
	CheckError(err)
	// Tempo artificial de acesso à CS
	time.Sleep(time.Second * 10)
	fmt.Println("Saí da CS")
}

// Rotina para desenfileirar processos
func dequeue() {
	for {
		if len(myQueue) > 0 {
			mutexTokenQueue.Lock()
			if myId == holder {
				// Se tem alguém na fila e possuo token
				entry := myQueue[0]
				if entry == myId {
					// Se for o próprio processo na fila, acessar a CS
					myQueue = myQueue[1:]
					accessSharedResource()
				} else {
					// Caso contrário, enviar o token
					holder = entry
					myQueue = myQueue[1:]
					go doClientJob(addrMap[holder], "TOKEN")
					if len(myQueue) > 0 {
						// Se ainda há processos na fila, pedir de volta o token
						forwardRequest = true
						go doClientJob(addrMap[holder], "REQUEST")
					}
				}
			} else if !forwardRequest {
				// Se ainda não tiver encaminhado requisição, encaminhar
				forwardRequest = true
				go doClientJob(addrMap[holder], "REQUEST")
			}
			mutexTokenQueue.Unlock()
		}
	}
}

func doServerJob() {

	buf := make([]byte, 1024)
	// Ler da conexão UDP a mensagem no buffer e tomar as devidas medidas em caso de receber REQUEST ou RELEASE
	for {
		n, addr, err := ServerConn.ReadFromUDP(buf)
		CheckError(err)
		msg := string(buf[0:n])
		fmt.Println("Received "+msg+" from", addr)
		if msg[0:5] == "TOKEN" {
			// Se recebeu token, guardar esta informação
			holder = myId
			forwardRequest = false
		} else if msg[0:7] == "REQUEST" {
			// Se recebeu request, enfileirar se ainda não tiver recebido request desse ID
			mutexTokenQueue.Lock()
			if !idInQueue(revPortMap[addr.Port], myQueue) {
				myQueue = append(myQueue, revPortMap[addr.Port])
			}
			mutexTokenQueue.Unlock()
		}
	}
}

// Rotina para enviar mensagens para outros processos
func doClientJob(otherProcessAddress *net.UDPAddr, msg string) {
	fmt.Println("Sent "+msg+" to ", otherProcessAddress)
	msgCounter++
	buf := make([]byte, 1024)
	buf = []byte(msg)
	_, err := ServerConn.WriteToUDP(buf, otherProcessAddress)
	CheckError(err)
}

// Método para iniciar as conexões entre todos os processos
func initConnections() {

	addrMap = make(map[int]*net.UDPAddr)
	revPortMap = make(map[int]int)

	ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+myPort)
	CheckError(err)
	ServerConn, err = net.ListenUDP("udp", ServerAddr)
	CheckError(err)

	// Estabelecer conexões com vizinhos
	for i := 0; i < 3; i++ {
		if os.Args[2*i+4] != "NULL" {
			id, err := strconv.Atoi(os.Args[2*i+4])
			CheckError(err)
			ServerAddr, err = net.ResolveUDPAddr("udp", "127.0.0.1"+os.Args[2*i+5])
			CheckError(err)
			addrMap[id] = ServerAddr
			revPortMap[ServerAddr.Port] = id
		}
	}
}

// Verificação se certo ID está na fila
func idInQueue(a int, list []int) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// A entrada para o processo será da forma ./Process {holder} {myId} {myPort} {parent_id} {parent_port} {left_id} {left_port} {right_id} {right_port}
func main() {

	msgCounter = 0
	// Inicialização das variáveis
	h, err := strconv.Atoi(os.Args[1])
	holder = h
	id, err := strconv.Atoi(os.Args[2])
	CheckError(err)
	myId = id
	myPort = os.Args[3]
	myQueue = make([]int, 0)

	// Iniciar conexões
	initConnections()

	// O fechamento de conexões ocorrerá quando a main retornar
	defer ServerConn.Close()

	ch := make(chan string) // Canal que registra caracteres digitados
	go readInput(ch)        // Chamar rotina que ”escuta” o teclado

	// Escutar outros processos
	go doServerJob()
	// Tentar desenfileirar
	go dequeue()

	for {
		// Verificar (de forma não bloqueante) se tem algo no stdin (input do terminal)
		select {
		case x, valid := <-ch:
			if valid {
				fmt.Printf("Recebi do teclado: %s \n", x)
				// Se a mensagem for "x", então pediu acesso à CS
				if x == "x" {
					// Se recebeu x e não está na fila, enfileirar
					mutexTokenQueue.Lock()
					if !idInQueue(myId, myQueue) {
						myQueue = append(myQueue, myId)
					}
					mutexTokenQueue.Unlock()

				} else if x == "y" {
					fmt.Println(msgCounter)
				} else if x == "z" {
					msgCounter = 0
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
