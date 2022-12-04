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

var msgCounter int

// Variáveis globais interessantes para o processo
var err string            // String de erro no caso de falha em algum procedimento
var myPort string         // Porta do meu servidor
var nextAddr *net.UDPAddr // Endereço do próximo na topologia anel
var nextId int            // Id do próximo
var nextConn *net.UDPConn // Conexão com o próximo

var myId int       // Identidade deste processo
var requested bool // Requisitou token
var hasToken bool  // Possui o token atualmente

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

// Rotina para tentar acessar o recurso compartilhado
func tryAccessSharedResource() {
	for {
		// Acessar a CS apenas quando tiver requisitado e tiver o token
		for !(requested && hasToken) {

		}
		fmt.Println("Entrei na CS")
		// Tempo artificial de acesso à CS
		time.Sleep(time.Second * 5)
		ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+":10001")
		CheckError(err)
		// Enviar mensagem com ID e um texto básico
		_, err = ServerConn.WriteToUDP([]byte("ID: "+strconv.Itoa(myId)+"\nACESSEI A CS\n"), ServerAddr)
		CheckError(err)
		fmt.Println("Saí da CS")
		// Acessou CS, não está mais requisitando
		requested = false
	}
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
			hasToken = true
			fwd_msg := msg[0:6]
			msgCounter, err = strconv.Atoi(msg[6:])
			CheckError(err)
			msgCounter++
			fwd_msg += strconv.Itoa(msgCounter)
			// Se recebeu token, enquanto está requisitado o token neste processo, esperar o término do acesso à CS
			for requested {

			}
			hasToken = false
			doClientJob(nextAddr, fwd_msg)
		}
	}
}

// Rotina para enviar mensagens para outros processos
func doClientJob(nextProcessAddr *net.UDPAddr, msg string) {
	time.Sleep(2 * time.Second)
	msgCounter++
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

	msgCounter = 0
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
	go tryAccessSharedResource()

	if hasToken {
		hasToken = false
		doClientJob(nextAddr, "TOKEN 1")
	}

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

}
