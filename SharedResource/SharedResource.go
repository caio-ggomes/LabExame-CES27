package main

import (
	"fmt"
	"net"
	"os"
)

/* Método para checar se há erro e apresentá-lo */
func CheckError(err error) {
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(0)
	}
}

func main() {
	/* Preparando o servidor na porta 10001 */
	ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+":10001")
	CheckError(err)

	/* Escutar */
	ServerConn, err := net.ListenUDP("udp", ServerAddr)
	CheckError(err)
	defer ServerConn.Close()

	buf := make([]byte, 1024)

	// Loop para ficar lendo o buffer e imprimindo as mensagens que chegarem
	for {
		n, addr, err := ServerConn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("Error: ", err)
		}
		fmt.Println("Received\n"+string(buf[0:n])+"from", addr)
	}
}
