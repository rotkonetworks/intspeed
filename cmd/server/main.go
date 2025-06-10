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

	var rootCmd = &cobra.Command{
		Use:   "intspeed-server",
		Short: "international speedtest web server",
		Run: func(cmd *cobra.Command, args []string) {
			runServer(port)
		},
	}

	rootCmd.Flags().IntVarP(&port, "port", "p", 8080, "Server port")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func runServer(port int) {
	server := web.NewServer()
	
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      server.Routes(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		fmt.Printf("🌐 intspeed server starting on port %d\n", port)
		fmt.Printf("📱 Open http://localhost:%d in your browser\n", port)
		
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	fmt.Println("\n🛑 Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
