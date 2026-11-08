package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"gitlab.com/vk-golang/lectures/seminars/01_reddit_clone/redditclone/internal/jwtconfig"
	"gitlab.com/vk-golang/lectures/seminars/01_reddit_clone/redditclone/internal/server"
	"gitlab.com/vk-golang/lectures/seminars/01_reddit_clone/redditclone/internal/store"
)

func main() {
	var wg sync.WaitGroup

	userStore := store.NewUserStore()
	postStore := store.NewPostStore()

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET is not set")
	}
	jwtManager := jwtconfig.NewManager([]byte(secret))

	server := server.NewServer(":8080", userStore, postStore, jwtManager)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	wg.Add(1)
	go func() {
		defer wg.Done()

		log.Println("Сервер запущен на :8080")

		err := server.Start()
		if err != nil {
			log.Printf("Ошибка сервера: %v", err)
			quit <- syscall.SIGTERM
		}
	}()

	<-quit
	log.Println("Получен сигнал завершения")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Сервер был вынужден завершиться: %v", err)
	}

	wg.Wait()

	log.Println("Сервер остановлен")
}
