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
var nextAddr *net.UDPAddr
var nextId int
var nextConn *net.UDPConn

var myId int       // Identidade deste processo
var requested bool // Requisitou token
var hasToken bool

var mutexRequest sync.Mutex // Mutex para garantir correta manipulação da variável request

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
	time.Sleep(time.Second * 5)
	ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+":10001")
	CheckError(err)
	// Enviar mensagem com ID e TIMESTAMP, além de um texto básico
	_, err = ServerConn.WriteToUDP([]byte("ID: "+strconv.Itoa(myId)+"\nACESSEI A CS\n"), ServerAddr)
	CheckError(err)
	fmt.Println("Saí da CS")
	requested = false
}

func doServerJob() {

	buf := make([]byte, 1024)
	for {
		// Ler da conexão UDP a mensagem no buffer
		n, addr, err := ServerConn.ReadFromUDP(buf)
		CheckError(err)
		msg := string(buf[0:n])
		fmt.Println("Received "+msg+" from ", addr)
		if msg[0:5] == "TOKEN" {
			if requested {
				hasToken = true
				go accessSharedResource()
			} else {
				go doClientJob(nextAddr, "TOKEN")
			}
		} else if msg[0:7] == "REQUEST" {
			if hasToken {
				hasToken = false
				for requested {
					time.Sleep(time.Millisecond * 500)
				}
				go doClientJob(nextAddr, "TOKEN")
			} else {
				go doClientJob(nextAddr, "REQUEST")
			}
		}
	}
}

// Rotina para enviar mensagens para outros processos
func doClientJob(nextProcessAddr *net.UDPAddr, msg string) {
	buf := make([]byte, 1024)
	buf = []byte(msg)
	_, err := ServerConn.WriteToUDP(buf, nextProcessAddr)
	CheckError(err)
}

// Método para iniciar as conexões entre todos os processos
func initConnections() {

	ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+myPort)
	CheckError(err)
	ServerConn, err = net.ListenUDP("udp", ServerAddr)
	CheckError(err)

	nextAddr, err = net.ResolveUDPAddr("udp", "127.0.0.1"+os.Args[5])
	CheckError(err)
	nextConn, err = net.DialUDP("udp", nil, nextAddr)
	CheckError(err)
}

// A entrada será da forma ./Process {has_token} {myId} {myPort} {nextId} {nextPort}
func main() {

	// Inicialização das variáveis
	t, err := strconv.ParseBool(os.Args[1])
	CheckError(err)
	hasToken = t
	id, err := strconv.Atoi(os.Args[2])
	CheckError(err)
	myId = id
	myPort = os.Args[3]

	nextId, err = strconv.Atoi(os.Args[4])
	CheckError(err)

	// Iniciar conexões
	initConnections()

	// O fechamento de conexões ocorrerá quando a main retornar
	defer ServerConn.Close()
	defer nextConn.Close()

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
					if requested {
						fmt.Println("x ignorado")
					} else {
						requested = true
						if hasToken {
							go accessSharedResource()
						} else {
							go doClientJob(nextAddr, "REQUEST")
						}
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
