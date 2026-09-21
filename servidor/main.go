// Servidor de cálculos. Cada conexão é tratada em uma goroutine própria.
//
// Uso:
//
//	go run ./servidor -porta 9001
//	go run ./servidor -porta 9002 -nome maquina-B
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"sort"
	"time"

	"calcdistribuida/protocolo"
)

func main() {
	porta := flag.Int("porta", 9001, "porta TCP em que o servidor escuta")
	nome := flag.String("nome", "", "nome do servidor (padrão: servidor-<porta>)")
	flag.Parse()

	if *nome == "" {
		*nome = fmt.Sprintf("servidor-%d", *porta)
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", *porta))
	if err != nil {
		log.Fatalf("erro ao escutar na porta %d: %v", *porta, err)
	}
	defer ln.Close()
	log.Printf("[%s] aguardando conexões na porta %d", *nome, *porta)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("erro no Accept: %v", err)
			continue
		}
		go tratarConexao(conn, *nome)
	}
}

func tratarConexao(conn net.Conn, nome string) {
	defer conn.Close()
	leitor := bufio.NewReader(conn)
	origem := conn.RemoteAddr().String()

	for {
		linha, err := leitor.ReadBytes('\n')
		if len(linha) > 0 {
			resp := processar(linha, nome, origem)
			dados, merr := json.Marshal(resp)
			if merr != nil {
				dados, _ = json.Marshal(protocolo.Resposta{
					ID:       resp.ID,
					Status:   "erro",
					Servidor: nome,
					Erro:     "falha ao serializar resposta: " + merr.Error(),
				})
			}
			dados = append(dados, '\n')
			if _, werr := conn.Write(dados); werr != nil {
				log.Printf("erro ao responder %s: %v", origem, werr)
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("erro de leitura de %s: %v", origem, err)
			}
			return
		}
	}
}

func processar(linha []byte, nome, origem string) protocolo.Resposta {
	t0 := time.Now()
	resp := protocolo.Resposta{Servidor: nome, Status: "ok"}

	var req protocolo.Requisicao
	if err := json.Unmarshal(linha, &req); err != nil {
		resp.Status = "erro"
		resp.Erro = "JSON inválido: " + err.Error()
		return resp
	}
	resp.ID = req.ID

	resultado, err := executar(req)
	if err != nil {
		resp.Status = "erro"
		resp.Erro = err.Error()
	} else {
		resp.Resultado = resultado
	}

	resp.TempoMs = float64(time.Since(t0).Microseconds()) / 1000
	log.Printf("[%s] %s (id %d) de %s -> %s em %.2f ms",
		nome, req.Operacao, req.ID, origem, resp.Status, resp.TempoMs)
	return resp
}

func executar(req protocolo.Requisicao) (map[string]float64, error) {
	switch req.Operacao {
	case protocolo.OpContarPrimos:
		if req.Inicio < 0 || req.Fim < req.Inicio {
			return nil, errors.New("intervalo inválido")
		}
		q := contarPrimos(req.Inicio, req.Fim)
		return map[string]float64{"quantidade": float64(q)}, nil

	case protocolo.OpBasicas:
		if len(req.Numeros) == 0 {
			return nil, errors.New("lista de números vazia")
		}
		return estatisticasBasicas(req.Numeros), nil

	case protocolo.OpAvancadas:
		if len(req.Numeros) == 0 {
			return nil, errors.New("lista de números vazia")
		}
		return estatisticasAvancadas(req.Numeros), nil

	default:
		return nil, fmt.Errorf("operação desconhecida: %q", req.Operacao)
	}
}

func ehPrimo(n int64) bool {
	if n < 2 {
		return false
	}
	if n < 4 {
		return true
	}
	if n%2 == 0 || n%3 == 0 {
		return false
	}
	for i := int64(5); i*i <= n; i += 6 {
		if n%i == 0 || n%(i+2) == 0 {
			return false
		}
	}
	return true
}

func contarPrimos(a, b int64) int64 {
	if a < 2 {
		a = 2
	}
	var total int64
	for n := a; n <= b; n++ {
		if ehPrimo(n) {
			total++
		}
	}
	return total
}

func estatisticasBasicas(nums []float64) map[string]float64 {
	soma := 0.0
	menor, maior := nums[0], nums[0]
	for _, v := range nums {
		soma += v
		if v < menor {
			menor = v
		}
		if v > maior {
			maior = v
		}
	}
	return map[string]float64{
		"quantidade": float64(len(nums)),
		"soma":       soma,
		"media":      soma / float64(len(nums)),
		"minimo":     menor,
		"maximo":     maior,
	}
}

func estatisticasAvancadas(nums []float64) map[string]float64 {
	n := float64(len(nums))

	ordenado := append([]float64(nil), nums...)
	sort.Float64s(ordenado)
	meio := len(ordenado) / 2
	var mediana float64
	if len(ordenado)%2 == 1 {
		mediana = ordenado[meio]
	} else {
		mediana = (ordenado[meio-1] + ordenado[meio]) / 2
	}

	soma := 0.0
	for _, v := range nums {
		soma += v
	}
	media := soma / n

	acc := 0.0
	for _, v := range nums {
		d := v - media
		acc += d * d
	}
	variancia := acc / n

	return map[string]float64{
		"mediana":       mediana,
		"variancia":     variancia,
		"desvio_padrao": math.Sqrt(variancia),
	}
}
