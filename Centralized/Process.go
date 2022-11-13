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
var myPort string          // Porta do meu servidor
var nServers int           // Quantidade de outros processos
var CliConn []*net.UDPConn // Vetor com conexões para os servidores dos outros processos

var myLogicalClock int // Relógio Lógico deste processo
var myId int           // Identidade deste processo
var myState string     // Atual Estado deste processo (RELEASED, WANTED ou HELD)
var myReplyCount int   // Número de Replies que este processo recebeu desde que ficou WANTED
var myQueue []int      // Fila de processos que pediram Request e têm prioridade menor que este processo

var mutexLogicalClock sync.Mutex // Mutex para garantir correta manipulação da variável Relógio Lógico
var mutexState sync.Mutex        // Mutex para garantir correta manipulação da variável Estado
var mutexReplyCount sync.Mutex   // Mutex para garantir correta manipulação da variável Número de Replies
var mutexQueue sync.Mutex        // Mutex para garantir correta manipulação da variável Fila

var ServConn *net.UDPConn // Conexão do servidor deste processo(onde são recebidas mensagens dos outros processos)

// Rotina para ler a entrada do processo e capturar os caracteres
func readInput(ch chan string) {
	reader := bufio.NewReader(os.Stdin)
	for {
		text, _, _ := reader.ReadLine()
		ch <- string(text)
	}
}

// Método para incrementar o relógio lógico
func incrementLogicalClock() {
	mutexLogicalClock.Lock()
	myLogicalClock++
	mutexLogicalClock.Unlock()
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
	time.Sleep(time.Second * 1)
	ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+":10001")
	CheckError(err)
	Conn, err := net.DialUDP("udp", nil, ServerAddr)
	CheckError(err)
	// Enviar mensagem com ID e TIMESTAMP, além de um texto básico
	_, err = Conn.Write([]byte("ID: " + strconv.Itoa(myId) + "\nTIMESTAMP: " + strconv.Itoa(myLogicalClock) + "\nACESSEI A CS\n"))
	CheckError(err)
	Conn.Close()
	fmt.Println("Saí da CS")
	mutexState.Lock()
	myState = "RELEASED"
	mutexState.Unlock()
	mutexQueue.Lock()
	// Desenfileirar os processos que ficaram na fila enquanto o processo estava HELD
	for j := 0; j < len(myQueue); j++ {
		pos := myQueue[j] - 1
		if myQueue[j] > myId {
			pos--
		}
		go doClientJob(pos, "REPLY:\nTIMESTAMP = "+strconv.Itoa(myLogicalClock)+"\n")
	}
	myQueue = make([]int, 0)
	mutexQueue.Unlock()
}

func doServerJob() {

	buf := make([]byte, 1024)
	for {
		// Ler da conexão UDP a mensagem no buffer e tomar as devidas medidas em caso de receber REQUEST ou REPLY
		n, addr, err := ServConn.ReadFromUDP(buf)
		CheckError(err)
		msg := string(buf[0:n])
		fmt.Println("Received\n"+msg+"from", addr)
		// Se a mensagem for reply
		if msg[0:5] == "REPLY" {
			// Capturar o timestamp encaminhado na mensagem
			brk := false
			timestampString := ""
			for i := 0; i < n && !brk; i++ {
				if msg[i] == '\n' {
					i += 13
					for ; i < n && !brk; i++ {
						if msg[i] == '\n' {
							brk = true
						} else {
							timestampString += string(msg[i])
						}
					}
				}
			}
			timestamp, err := strconv.Atoi(timestampString)
			CheckError(err)
			mutexLogicalClock.Lock()
			// Verificar se é necessária uma atualização do relógio lógico
			if myLogicalClock < timestamp {
				myLogicalClock = timestamp
			}
			mutexLogicalClock.Unlock()
			incrementLogicalClock()
			mutexReplyCount.Lock()
			myReplyCount += 1
			mutexReplyCount.Unlock()
			// Se tiver recebido todos os replies, ficar HELD e acessar a CS
			if myReplyCount == nServers {
				mutexReplyCount.Lock()
				myReplyCount = 0
				mutexReplyCount.Unlock()
				mutexState.Lock()
				myState = "HELD"
				mutexState.Unlock()
				go accessSharedResource()
			}
		} else if msg[0:7] == "REQUEST" {
			// Já se foi um request, capturar o timestamp e o id da mensagem
			brk := false
			timestampString := ""
			idString := ""
			for i := 0; i < n && !brk; i++ {
				if msg[i] == '\n' {
					i += 6
					for ; i < n && !brk; i++ {
						if msg[i] == '\n' {
							i += 13
							for ; i < n && !brk; i++ {
								if msg[i] == '\n' {
									brk = true
								} else {
									timestampString += string(msg[i])
								}
							}
						} else {
							idString += string(msg[i])
						}
					}
				}
			}
			timestamp, err := strconv.Atoi(timestampString)
			CheckError(err)
			id, err := strconv.Atoi(idString)
			CheckError(err)
			mutexLogicalClock.Lock()
			// Verificação de atualização do relógio lógico
			if myLogicalClock < timestamp {
				myLogicalClock = timestamp
			}
			mutexLogicalClock.Unlock()
			incrementLogicalClock()
			// Se HELD ou WANTED e tem precedência menor, pelo menor timestamp, então enfileirar
			if myState == "HELD" || (myState == "WANTED" && timestamp < myLogicalClock) {
				mutexQueue.Lock()
				myQueue = append(myQueue, id)
				mutexQueue.Unlock()
			} else {
				// Caso contrário, dar reply
				pos := id - 1
				if id > myId {
					pos--
				}
				go doClientJob(pos, "REPLY:\nTIMESTAMP = "+strconv.Itoa(myLogicalClock)+"\n")
			}
		}
	}
}

// Rotina para enviar mensagens para outros processos
func doClientJob(otherProcess int, msg string) {
	buf := make([]byte, 1024)
	buf = []byte(msg)
	_, err := CliConn[otherProcess].Write(buf)
	CheckError(err)
}

// Método para iniciar as conexões entre todos os processos
func initConnections() {
	myPort = os.Args[2]
	nServers = len(os.Args) - 3
	/*Esse -3 tira o nome (no caso ./Process), o Id deste processo, e a porta
	deste processo. As demais portas são dos outros processos*/

	CliConn = make([]*net.UDPConn, nServers)

	ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+myPort)
	CheckError(err)
	ServConn, err = net.ListenUDP("udp", ServerAddr)
	CheckError(err)

	/*Conexões com os servidores de cada processo. Colocar tais conexões no vetor CliConn.*/
	for servidores := 0; servidores < nServers; servidores++ {
		ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+os.Args[3+servidores])
		CheckError(err)
		Conn, err := net.DialUDP("udp", nil, ServerAddr)
		CliConn[servidores] = Conn
		CheckError(err)
	}
}

// A entrada será da forma ./Process {myId} {myPort} {Port_1} {Port_2} ... {Port_n}
func main() {

	// Inicialização das variáveis
	id, err := strconv.Atoi(os.Args[1])
	myId = id
	CheckError(err)
	mutexLogicalClock.Lock()
	myLogicalClock = 0
	mutexLogicalClock.Unlock()
	mutexState.Lock()
	myState = "RELEASED"
	mutexState.Unlock()
	mutexReplyCount.Lock()
	myReplyCount = 0
	mutexReplyCount.Unlock()
	mutexQueue.Lock()
	myQueue = make([]int, 0)
	mutexQueue.Unlock()

	// Iniciar conexões
	initConnections()

	// O fechamento de conexões ocorrerá quando a main retornar
	defer ServConn.Close()
	for i := 0; i < nServers; i++ {
		defer CliConn[i].Close()
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
					if myState == "WANTED" || myState == "HELD" {
						fmt.Println("x ignorado")
					} else {
						mutexState.Lock()
						myState = "WANTED"
						mutexState.Unlock()
						incrementLogicalClock()
						// Multicast para todos os processos do Request
						for j := 0; j < nServers; j++ {
							go doClientJob(j, "REQUEST:\nID = "+strconv.Itoa(myId)+"\nTIMESTAMP = "+strconv.Itoa(myLogicalClock)+"\n")
						}
					}
				} else if x == strconv.Itoa(myId) {
					// Se a entrada for o ID, executar processo interno que incrementa o relógio lógico
					incrementLogicalClock()
				}
			} else {
				fmt.Println("Canal fechado!")
			}
		default:
			// Fazer nada!
			// Mas não fica bloqueado esperando o teclado
			time.Sleep(time.Second * 1)
		}

		// Espera um pouco
		time.Sleep(time.Second * 1)
	}
}
