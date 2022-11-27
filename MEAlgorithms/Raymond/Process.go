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

var accessed bool

var leftConn *net.UDPConn
var rightConn *net.UDPConn
var parentConn *net.UDPConn

var addrMap map[int]*net.UDPAddr
var revPortMap map[int]int

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
	// Enviar mensagem com ID e TIMESTAMP, além de um texto básico
	_, err = ServerConn.WriteToUDP([]byte("ID: "+strconv.Itoa(myId)+"\nACESSEI A CS\n"), ServerAddr)
	CheckError(err)
	time.Sleep(time.Second * 5)
	fmt.Println("Saí da CS")
	accessed = false
}

func doServerJob() {

	buf := make([]byte, 1024)
	// Ler da conexão UDP a mensagem no buffer e tomar as devidas medidas em caso de receber REQUEST ou RELEASE
	for {
		n, addr, err := ServerConn.ReadFromUDP(buf)
		CheckError(err)
		msg := string(buf[0:n])
		fmt.Println("Received "+msg+" from", addr)
		for accessed {
			time.Sleep(500 * time.Millisecond)
		}
		if msg[0:5] == "TOKEN" {
			holder = myId
			entry := myQueue[0]
			myQueue = myQueue[1:]
			if entry == myId {
				accessed = true
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
				holder = entry
				go doClientJob(addrMap[entry], "TOKEN")
				if len(myQueue) > 0 {
					go doClientJob(addrMap[holder], "REQUEST")
				}
			}
		} else if msg[0:7] == "REQUEST" {
			if holder == myId {
				holder = revPortMap[addr.Port]
				go doClientJob(addrMap[holder], "TOKEN")
			} else {
				myQueue = append(myQueue, revPortMap[addr.Port])
				go doClientJob(addrMap[holder], "REQUEST")
			}
		}
	}
}

// Rotina para enviar mensagens para outros processos
func doClientJob(otherProcessAddress *net.UDPAddr, msg string) {
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

	// Inicialização das variáveis
	h, err := strconv.Atoi(os.Args[1])
	holder = h
	id, err := strconv.Atoi(os.Args[2])
	CheckError(err)
	myId = id
	myPort = os.Args[3]
	myQueue = make([]int, 0)

	accessed = false

	// Iniciar conexões
	initConnections()

	// O fechamento de conexões ocorrerá quando a main retornar
	defer ServerConn.Close()

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
					for accessed {
						time.Sleep(500 * time.Millisecond)
					}
					if !idInQueue(myId, myQueue) && myId != holder {
						myQueue = append(myQueue, myId)
						go doClientJob(addrMap[holder], "REQUEST")
					} else if !idInQueue(myId, myQueue) && myId == holder {
						myQueue = append(myQueue, myId)
						accessed = true
						go accessSharedResource()
						myQueue = myQueue[1:]
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
