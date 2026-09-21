# Cálculo distribuído com sockets TCP em Go

Um cliente em modo texto distribui o processamento entre **2 ou mais servidores**.
Cada servidor trata cada conexão em uma goroutine. As mensagens são strings **JSON**,
uma por linha (`\n`), sobre TCP.

## Estrutura

```
calcdistribuida/
├── go.mod
├── protocolo/protocolo.go   # structs JSON + função Enviar (usadas pelo cliente)
├── servidor/main.go         # servidor TCP (uma goroutine por conexão)
└── cliente/main.go          # cliente em modo texto
```

## Como executar

Abra um terminal por servidor (na pasta `calcdistribuida`):

```bash
go run ./servidor -porta 9001
go run ./servidor -porta 9002
go run ./servidor -porta 9003
```

Em outro terminal, rode o cliente:

```bash
go run ./cliente
```

Para usar computadores diferentes, informe os endereços:

```bash
go run ./cliente -servidores "192.168.0.10:9001,192.168.0.11:9001"
```

O cliente verifica quais servidores estão acessíveis e exige pelo menos 2.

## Operações e como a carga é distribuída

| Opção | O que faz | Distribuição |
|---|---|---|
| 1 | Conta primos em `[início, fim]` | O intervalo é dividido em blocos (4 por servidor). Cada servidor tem uma goroutine no cliente que pega o próximo bloco livre da fila, então servidores mais rápidos processam mais blocos. O cliente soma os parciais. |
| 2 | Estatísticas de uma lista | Operações **diferentes** em servidores **diferentes**: um calcula soma/média/mín/máx, outro calcula mediana/variância/desvio padrão. O cliente junta os resultados. |

## Protocolo (JSON)

Requisição:

```json
{"id":1,"operacao":"contar_primos","inicio":1,"fim":1000}
{"id":2,"operacao":"estatisticas_basicas","numeros":[3,1.5,8]}
```

Resposta (mesmo formato, JSON):

```json
{"id":1,"status":"ok","servidor":"servidor-9001","resultado":{"quantidade":168},"tempo_ms":0.42}
{"id":9,"status":"erro","servidor":"servidor-9001","erro":"operação desconhecida: \"xyz\"","tempo_ms":0}
```

Operações: `contar_primos`, `estatisticas_basicas`, `estatisticas_avancadas`.

## Testando um servidor sozinho

```bash
echo '{"id":1,"operacao":"contar_primos","inicio":1,"fim":100}' | nc localhost 9001
# esperado: quantidade = 25
```
