package mtr

import (
	"fmt"
	tm "github.com/buger/goterm"
	"github.com/google/gops/agent"
	"github.com/spf13/cobra"
	"log"
	"sync"
	"testing"
	"time"
)

var (
	version string
	date    string

	COUNT            = 5
	TIMEOUT          = 200 * time.Millisecond
	INTERVAL         = 50 * time.Millisecond
	HOP_SLEEP        = time.Nanosecond
	MAX_HOPS         = 30
	MAX_UNKNOWN_HOPS = 50
	RING_BUFFER_SIZE = 50
	PTR_LOOKUP       = true
	jsonFmt          = false
	srcAddr          = "0.0.0.0"
	versionFlag      bool
)

func TestName(t *testing.T) {
	var RootCmd = &cobra.Command{
		Use:  "mtr TARGET",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if versionFlag {
				fmt.Printf("MTR Version: %s, build date: %s\n", version, date)
				return nil
			}
			m, ch, err := NewMTR(args[0], srcAddr, TIMEOUT, INTERVAL, HOP_SLEEP,
				MAX_HOPS, MAX_UNKNOWN_HOPS, RING_BUFFER_SIZE, PTR_LOOKUP)
			if err != nil {
				return err
			}
			if jsonFmt {
				go func(ch chan struct{}) {
					for {
						<-ch
					}
				}(ch)
				m.Run(ch, COUNT)
				//s, err := pj.Marshal(m)
				if err != nil {
					return err
				}
				//fmt.Println(string(s))
				return nil
			}
			fmt.Println("Start:", time.Now())
			mu := &sync.Mutex{}
			go func(ch chan struct{}) {
				for {
					mu.Lock()
					<-ch
					//fmt.Println(m)
					mu.Unlock()
				}
			}(ch)
			m.Run(ch, COUNT)
			close(ch)
			mu.Lock()
			//
			mu.Unlock()
			return nil
		},
	}
	//RootCmd.SetArgs([]string{"39.156.66.10"})
	RootCmd.SetArgs([]string{"172.253.122.147"})

	RootCmd.Execute()
}
func render(m *MTR) {
	tm.MoveCursor(1, 1)
	m.Render(1)
	tm.Flush() // Call it every time at the end of rendering
}
func init() {
	if err := agent.Listen(agent.Options{}); err != nil {
		log.Fatal(err)
	}
}

func TestKkkk(t *testing.T) {

	m, ch, err := NewMTR("39.156.66.10", srcAddr, TIMEOUT, INTERVAL, HOP_SLEEP,
		MAX_HOPS, MAX_UNKNOWN_HOPS, RING_BUFFER_SIZE, PTR_LOOKUP)
	if err != nil {
		fmt.Println(err)
	}
	//m.Run(ch, COUNT)
	mu := &sync.Mutex{}
	go func(ch chan struct{}) {
		for {
			mu.Lock()
			<-ch
			//fmt.Println(m)
			mu.Unlock()
		}
	}(ch)
	m.Run(ch, COUNT)
	close(ch)
	mu.Lock()
	for _, d := range m.StringResult() {
		fmt.Print(d)
	}
	mu.Unlock()
}
