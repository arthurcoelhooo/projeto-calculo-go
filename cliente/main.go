// Cliente em modo texto. Distribui o processamento entre 2 ou mais servidores.
//
// Uso:
//
//	go run ./cliente
//	go run ./cliente -servidores "192.168.0.10:9001,192.168.0.11:9001"
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"calcdistribuida/protocolo"
)

const timeoutReq = 2 * time.Minute

var (
	entrada = bufio.NewReader(os.Stdin)
	proximo int // rodízio para escolher servidores nas estatísticas
)

func main() {
	lista := flag.String("servidores",
		"localhost:9001,localhost:9002,localhost:9003",
		"endereços host:porta dos servidores, separados por vírgula")
	flag.Parse()

	fmt.Println("Verificando servidores...")
	var ativos []string
	for _, e := range strings.Split(*lista, ",") {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if protocolo.Alcancavel(e) {
			fmt.Printf("  [ok]  %s\n", e)
			ativos = append(ativos, e)
		} else {
			fmt.Printf("  [off] %s (inacessível)\n", e)
		}
	}
	if len(ativos) < 2 {
		fmt.Println("\nSão necessários pelo menos 2 servidores ativos para distribuir a carga.")
		os.Exit(1)
	}

	for {
		fmt.Println("\n=== Cálculo distribuído ===")
		fmt.Println("1) Contar números primos em um intervalo")
		fmt.Println("2) Estatísticas de uma lista de números")
		fmt.Println("0) Sair")

		switch lerLinha("Escolha: ") {
		case "1":
			opcaoPrimos(ativos)
		case "2":
			opcaoEstatisticas(ativos)
		case "0":
			return
		default:
			fmt.Println("Opção inválida.")
		}
	}
}

func lerLinha(prompt string) string {
	fmt.Print(prompt)
	s, err := entrada.ReadString('\n')
	if err != nil && s == "" {
		fmt.Println()
		os.Exit(0)
	}
	return strings.TrimSpace(s)
}

type bloco struct {
	id      int
	de, ate int64
}

func opcaoPrimos(servidores []string) {
	a, err := strconv.ParseInt(lerLinha("Início do intervalo: "), 10, 64)
	if err != nil {
		fmt.Println("Valor inválido.")
		return
	}
	b, err := strconv.ParseInt(lerLinha("Fim do intervalo: "), 10, 64)
	if err != nil || b < a || a < 0 {
		fmt.Println("Intervalo inválido.")
		return
	}
	contarPrimosDistribuido(servidores, a, b)
}

func contarPrimosDistribuido(servidores []string, a, b int64) {
	total := b - a + 1
	nBlocos := int64(len(servidores) * 4)
	if nBlocos > total {
		nBlocos = total
	}
	tam := (total + nBlocos - 1) / nBlocos

	var blocos []bloco
	for ini, id := a, 1; ini <= b; ini, id = ini+tam, id+1 {
		fim := ini + tam - 1
		if fim > b {
			fim = b
		}
		blocos = append(blocos, bloco{id: id, de: ini, ate: fim})
	}

	fila := make(chan bloco, len(blocos))
	for _, bl := range blocos {
		fila <- bl
	}
	close(fila)

	var (
		mu          sync.Mutex
		wg          sync.WaitGroup
		totalPrimos float64
		porServidor = map[string]int{}
		falhas      []string
	)

	t0 := time.Now()
	for _, endereco := range servidores {
		wg.Add(1)
		go func(endereco string) {
			defer wg.Done()
			for bl := range fila {
				resp, err := protocolo.Enviar(endereco, protocolo.Requisicao{
					ID:       bl.id,
					Operacao: protocolo.OpContarPrimos,
					Inicio:   bl.de,
					Fim:      bl.ate,
				}, timeoutReq)

				mu.Lock()
				switch {
				case err != nil:
					falhas = append(falhas, fmt.Sprintf("bloco %d [%d,%d]: %v", bl.id, bl.de, bl.ate, err))
				case resp.Status != "ok":
					falhas = append(falhas, fmt.Sprintf("bloco %d [%d,%d]: %s", bl.id, bl.de, bl.ate, resp.Erro))
				default:
					totalPrimos += resp.Resultado["quantidade"]
					porServidor[resp.Servidor]++
				}
				mu.Unlock()
			}
		}(endereco)
	}
	wg.Wait()

	fmt.Printf("\nPrimos em [%d, %d]: %.0f", a, b, totalPrimos)
	if len(falhas) > 0 {
		fmt.Print("  (RESULTADO PARCIAL)")
	}
	fmt.Printf("\nTempo total: %v (%d blocos)\n", time.Since(t0).Round(time.Millisecond), len(blocos))

	fmt.Println("Blocos processados por servidor:")
	nomes := make([]string, 0, len(porServidor))
	for n := range porServidor {
		nomes = append(nomes, n)
	}
	sort.Strings(nomes)
	for _, n := range nomes {
		fmt.Printf("  %-20s %d\n", n, porServidor[n])
	}

	if len(falhas) > 0 {
		fmt.Printf("\nATENÇÃO: %d bloco(s) falharam:\n", len(falhas))
		for _, f := range falhas {
			fmt.Println("  -", f)
		}
	}
}

func opcaoEstatisticas(servidores []string) {
	linha := lerLinha("Números separados por espaço (aceita vírgula ou ponto decimal): ")

	var numeros []float64
	for _, tok := range strings.Fields(linha) {
		v, err := strconv.ParseFloat(strings.ReplaceAll(tok, ",", "."), 64)
		if err != nil {
			fmt.Printf("Valor inválido: %q\n", tok)
			return
		}
		numeros = append(numeros, v)
	}
	if len(numeros) == 0 {
		fmt.Println("Nenhum número informado.")
		return
	}

	ops := []string{protocolo.OpBasicas, protocolo.OpAvancadas}
	respostas := make([]protocolo.Resposta, len(ops))
	erros := make([]error, len(ops))

	var wg sync.WaitGroup
	for i, op := range ops {
		destino := servidores[(proximo+i)%len(servidores)]
		wg.Add(1)
		go func(i int, op, destino string) {
			defer wg.Done()
			respostas[i], erros[i] = protocolo.Enviar(destino, protocolo.Requisicao{
				ID:       i + 1,
				Operacao: op,
				Numeros:  numeros,
			}, timeoutReq)
		}(i, op, destino)
	}
	wg.Wait()
	proximo++

	fmt.Println("\nResultado combinado:")
	for i, r := range respostas {
		switch {
		case erros[i] != nil:
			fmt.Printf("  -- %s: FALHA (%v)\n", ops[i], erros[i])
		case r.Status != "ok":
			fmt.Printf("  -- %s: erro do servidor: %s\n", ops[i], r.Erro)
		default:
			fmt.Printf("  -- %s (calculado por %s em %.2f ms)\n", ops[i], r.Servidor, r.TempoMs)
			chaves := make([]string, 0, len(r.Resultado))
			for k := range r.Resultado {
				chaves = append(chaves, k)
			}
			sort.Strings(chaves)
			for _, k := range chaves {
				fmt.Printf("       %-15s = %g\n", k, r.Resultado[k])
			}
		}
	}
}
