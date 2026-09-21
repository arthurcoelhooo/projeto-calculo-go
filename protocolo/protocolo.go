package protocolo

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// Operações suportadas pelos servidores.
const (
	OpContarPrimos = "contar_primos"          // conta primos em [Inicio, Fim]
	OpBasicas      = "estatisticas_basicas"   // soma, média, mínimo, máximo, quantidade
	OpAvancadas    = "estatisticas_avancadas" // mediana, variância, desvio padrão
)

// Requisicao é a mensagem enviada do cliente para o servidor.
//
// Exemplo:
//
//	{"id":1,"operacao":"contar_primos","inicio":1,"fim":1000}
//	{"id":2,"operacao":"estatisticas_basicas","numeros":[3,1.5,8]}
type Requisicao struct {
	ID       int       `json:"id"`
	Operacao string    `json:"operacao"`
	Inicio   int64     `json:"inicio,omitempty"`
	Fim      int64     `json:"fim,omitempty"`
	Numeros  []float64 `json:"numeros,omitempty"`
}

// Resposta é a mensagem devolvida pelo servidor ao cliente.
//
// Exemplo:
//
//	{"id":1,"status":"ok","servidor":"servidor-9001","resultado":{"quantidade":168},"tempo_ms":0.42}
type Resposta struct {
	ID        int                `json:"id"`
	Status    string             `json:"status"` // "ok" ou "erro"
	Servidor  string             `json:"servidor"`
	Resultado map[string]float64 `json:"resultado,omitempty"`
	Erro      string             `json:"erro,omitempty"`
	TempoMs   float64            `json:"tempo_ms"`
}

// Alcancavel informa se é possível abrir uma conexão TCP com o endereço.
func Alcancavel(endereco string) bool {
	conn, err := net.DialTimeout("tcp", endereco, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// Enviar abre uma conexão com o servidor, envia a requisição em JSON e
// aguarda a resposta (também em JSON).
func Enviar(endereco string, req Requisicao, timeout time.Duration) (Resposta, error) {
	var resp Resposta

	conn, err := net.DialTimeout("tcp", endereco, 3*time.Second)
	if err != nil {
		return resp, fmt.Errorf("conectando a %s: %w", endereco, err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(timeout))

	dados, err := json.Marshal(req)
	if err != nil {
		return resp, fmt.Errorf("serializando requisição: %w", err)
	}
	dados = append(dados, '\n')
	if _, err := conn.Write(dados); err != nil {
		return resp, fmt.Errorf("enviando para %s: %w", endereco, err)
	}

	linha, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return resp, fmt.Errorf("lendo resposta de %s: %w", endereco, err)
	}
	if err := json.Unmarshal(linha, &resp); err != nil {
		return resp, fmt.Errorf("resposta inválida de %s: %w", endereco, err)
	}
	return resp, nil
}
