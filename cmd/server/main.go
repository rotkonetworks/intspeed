package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/rotkonetworks/intspeed/pkg/web"
	"github.com/spf13/cobra"
)

func main() {
	var port int
	var dir string

	var rootCmd = &cobra.Command{
		Use:   "intspeed-server",
		Short: "serves the intspeed browser frontend (tests run in the visitor's browser)",
		Run: func(cmd *cobra.Command, args []string) {
			runServer(port, dir)
		},
	}

	rootCmd.Flags().IntVarP(&port, "port", "p", 8080, "Server port")
	rootCmd.Flags().StringVarP(&dir, "dir", "d", "web", "Static assets directory")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func runServer(port int, dir string) {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      web.StaticHandler(dir),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	go func() {
		fmt.Printf("🌐 intspeed server on http://localhost:%d (serving %s)\n", port, dir)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	fmt.Println("\n🛑 Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
