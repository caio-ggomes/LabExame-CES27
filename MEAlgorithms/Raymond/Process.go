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
var err string
var myPort string // Porta do meu servidor

var myId int // Identidade deste processo

var leftConn *net.UDPConn
var rightConn *net.UDPConn
var parentConn *net.UDPConn

var addrMap map[int]*net.UDPAddr
var revAddrMap map[*net.UDPAddr]int
var connMap map[*net.UDPAddr]*net.UDPConn

var forwardRequest bool
var holder int
var myQueue []int // Fila de processos que pediram Request ao coordenador

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
	fmt.Println("Entrei na CS")
	ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+":10001")
	CheckError(err)
	Conn, err := net.DialUDP("udp", nil, ServerAddr)
	CheckError(err)
	// Enviar mensagem com ID e TIMESTAMP, além de um texto básico
	_, err = Conn.Write([]byte("ID: " + strconv.Itoa(myId) + "\nACESSEI A CS\n"))
	CheckError(err)
	time.Sleep(time.Second * 1)
	Conn.Close()
	fmt.Println("Saí da CS")
}

func doServerJob() {

	buf := make([]byte, 1024)
	// Ler da conexão UDP a mensagem no buffer e tomar as devidas medidas em caso de receber REQUEST ou RELEASE
	for {
		n, addr, err := ServerConn.ReadFromUDP(buf)
		CheckError(err)
		msg := string(buf[0:n])
		fmt.Println("Received\n"+msg+"from", addr)
		if msg[0:7] == "REQUEST" {
			if holder == myId {
				holder = revAddrMap[addr]
				go doClientJob(addrMap[holder], "TOKEN")
			} else {
				myQueue = append(myQueue, revAddrMap[addr])
				if !forwardRequest {
					forwardRequest = true
					go doClientJob(addrMap[holder], "REQUEST")
				}
			}
		} else if msg[0:5] == "TOKEN" {
			entry := myQueue[0]
			myQueue = myQueue[1:]
			if entry == myId {
				go accessSharedResource()
				if len(myQueue) > 0 {
					holder = myQueue[0]
					myQueue = myQueue[1:]
					go doClientJob(addrMap[holder], "TOKEN")
					if len(myQueue) > 0 {
						go doClientJob(addrMap[holder], "REQUEST")
					}
				}
			} else {
				go doClientJob(addrMap[entry], "TOKEN")
				if len(myQueue) > 0 {
					go doClientJob(addrMap[holder], "REQUEST")
				}
			}
		}
	}
}

// Rotina para enviar mensagens para outros processos
func doClientJob(otherProcessAddress *net.UDPAddr, msg string) {
	buf := make([]byte, 1024)
	buf = []byte(msg)
	_, err := connMap[otherProcessAddress].Write(buf)
	CheckError(err)
}

// Método para iniciar as conexões entre todos os processos
func initConnections() {

	//CliConn = make([]*net.UDPConn, nServers)

	ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+myPort)
	CheckError(err)
	ServerConn, err = net.ListenUDP("udp", ServerAddr)
	CheckError(err)

	for i := 0; i < 3; i++ {
		if os.Args[2*i+3] != "NULL" {
			id, err := strconv.Atoi(os.Args[3])
			CheckError(err)
			ServerAddr, err = net.ResolveUDPAddr("udp", "127.0.0.1"+os.Args[2*i+4])
			CheckError(err)
			addrMap[id] = ServerAddr
			revAddrMap[ServerAddr] = id
			Conn, err := net.DialUDP("udp", nil, ServerAddr)
			CheckError(err)
			connMap[ServerAddr] = Conn
		}
	}
}

// A entrada para o coordenador será da forma ./Process {myId} {myPort} {parent_id} {parent_port} {left_id} {left_port} {right_id} {right_port}
func main() {

	// Inicialização das variáveis
	id, err := strconv.Atoi(os.Args[1])
	CheckError(err)
	myId = id
	myPort = os.Args[2]
	myQueue = make([]int, 3)

	// Iniciar conexões
	initConnections()

	// O fechamento de conexões ocorrerá quando a main retornar
	defer ServerConn.Close()
	for _, conn := range connMap {
		defer conn.Close()
	}

	ch := make(chan string) // Canal que registra caracteres digitados
	go readInput(ch)        // Chamar rotina que ”escuta” o teclado

	// Escutar outros processos
	go doServerJob()

	for {
		// Verificar (de forma não bloqueante) se tem algo no stdin (input do terminal)
		select {
		case x, valid := <-ch:
			if valid {
				fmt.Printf("Recebi do teclado: %s \n", x)
				// Se a mensagem for "x", então pediu acesso à CS
				if x == "x" {
					// Pedido indevido
					if len(myQueue) == 0 && myId != holder {
						go doClientJob(addrMap[holder], "REQUEST")
						myQueue = append(myQueue, myId)
					} else {
						fmt.Println("x ignorado")
					}
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
