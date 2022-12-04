# Análise comparativa entre algoritmos de exclusão mútua

Laboratório de Exame-CES27

## Contribuidores

| [<img src="https://avatars.githubusercontent.com/u/54087165?v=4" width="115"><br><sub>@caio-ggomes</sub>](https://github.com/caio-ggomes) | [<img src="https://avatars.githubusercontent.com/u/80851723?v=4" width="115"><br><sub>@thomascastroneto</sub>](https://github.com/thomascastroneto) |
|:-:|:-:|

Comandos para execução

No diretório SharedResource
```{c}
$ go build SharedResource.go
./SharedResource
```

Nos demais diretórios dentro de MEAlgorithms
```{c}
$ go build Process.go
```

Execução do Algoritmo de Token Ring:
```{c}
./Process {has_token} {myId} {myPort} {nextId} {nextPort}
```

Execução do Algoritmo Centralizado (se for processo):
```{c}
./Process p {myId} {myPort} {Coordinator_id} {Coordinator_port}
```
ou (se for coordenador)
```{c}
./Process c {myId} {myPort}
```

Execução do Algoritmo de Raymond:
```{c}
./Process {holder} {myId} {myPort} {parent_id} {parent_port} {left_id} {left_port} {right_id} {right_port}
```
Caso não tenha filho ou pai colocar "NULL" em id e porta correspondente. {holder} diz respeito a qual vizinho está no caminho de quem é o detentor do token
