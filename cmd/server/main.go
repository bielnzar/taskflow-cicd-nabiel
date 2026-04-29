// Entry point aplikasi TaskFlow API.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/taskflow/api/internal/handler"
	"github.com/taskflow/api/internal/repository"
	"github.com/taskflow/api/internal/service"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Pilih repository berdasarkan DATABASE_URL.
	// Jika tidak ada → gunakan MemoryRepository (untuk development tanpa DB).
	var repo repository.TaskRepository
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		pgRepo, err := repository.NewPostgresRepository(dbURL)
		if err != nil {
			log.Fatalf("❌ Gagal konek ke database: %v\n"+
				"   Pastikan DATABASE_URL benar dan postgres berjalan.", err)
		}
		if err := pgRepo.Migrate(); err != nil {
			log.Fatalf("❌ Migrasi database gagal: %v", err)
		}
		repo = pgRepo
		log.Println("✅ Menggunakan PostgreSQL repository")
	} else {
		repo = repository.NewMemoryRepository()
		log.Println("⚠️  DATABASE_URL tidak di-set — menggunakan MemoryRepository")
		log.Println("   Data TIDAK akan persisten setelah server restart.")
	}
	defer repo.Close()

	svc := service.NewTaskService(repo)
	h := handler.New(svc)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	log.Printf("🚀 TaskFlow API berjalan di :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server gagal: %v", err)
	}
}
