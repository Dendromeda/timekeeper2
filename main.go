package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

var (
	defaultBaseUrl  = "http://localhost"
	defaultBasePort = "3333"
	defaultPassword = "password"
)

type site struct {
	BaseUrl  string
	BasePort string
	Password string
}

type timestamp struct {
	Id    string
	Stamp time.Time
}

type stampList struct {
	*sync.Mutex
	stamps []timestamp
}

var stamps *stampList

type sumCalc struct {
	*sync.Mutex
	m map[string]int
}

var sums sumCalc = sumCalc{&sync.Mutex{}, make(map[string]int)}

func main() {

	baseUrl := os.Getenv("TIMEKEEPER_BASE_URL")
	if baseUrl == "" {
		baseUrl = defaultBaseUrl
	}
	basePort := os.Getenv("TIMEKEEPER_BASE_PORT")
	if basePort == "" {
		basePort = defaultBasePort
	}
	password := os.Getenv("TIMEKEEPER_PASSWORD")
	if password == "" {
		password = defaultPassword
	}

	readStamps()

	st := site{baseUrl, basePort, password}

	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		// AllowedOrigins:   []string{"https://foo.com"}, // Use this to allow specific origin hosts
		AllowedOrigins: []string{"*"},
		// AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))
	r.Use(middleware.Logger)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		f, err := os.ReadFile("index.html")
		if err != nil {
			panic(err)
		}

		tmpl, err := template.New("index.html").Parse(string(f))
		if err != nil {
			panic(err)
		}

		tmpl.Execute(w, st)
	})
	r.Post("/trigger/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		password := r.URL.Query().Get("password")
		if password != st.Password {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if id == "Reset" {
			reset()
		} else {
			addTimestamp(id)
		}
		w.Write([]byte("OK"))
	})
	r.Get("/sums", func(w http.ResponseWriter, r *http.Request) {
		calculateSum()
		x, err := json.MarshalIndent(sums.m, "", "  ")
		if err != nil {
			panic(err)
		}
		w.Write(x)
	})

	port := "3333"

	if len(os.Args) > 1 {
		port = os.Args[1]
	}

	fmt.Println("Starting time tracker on " + baseUrl)
	err := http.ListenAndServe(":"+port, r)
	if err != nil {
		panic(err)
	}
}

func reset() {
	stamps.Lock()
	defer stamps.Unlock()
	stamps.stamps = []timestamp{}
	writeStamps()
}

func addTimestamp(id string) {
	stamps.Lock()
	defer stamps.Unlock()
	stamps.stamps = append(stamps.stamps, timestamp{id, time.Now()})
	writeStamps()
}

func calculateSum() {
	sums.Lock()
	defer sums.Unlock()
	m := make(map[string]time.Duration)
	var s1 timestamp
	if len(stamps.stamps) > 0 {
		s1 = stamps.stamps[0]
	}
	for i := 1; i < len(stamps.stamps); i++ {
		s2 := stamps.stamps[i]
		if _, ok := m[s1.Id]; !ok {
			m[s1.Id] = 0
		}
		m[s1.Id] += s2.Stamp.Sub(s1.Stamp)
		s1 = s2
	}
	m[s1.Id] += time.Since(s1.Stamp)

	sums.m = make(map[string]int)
	for k, v := range m {
		sums.m[k] = int(v.Seconds())
	}
}

func writeStamps() {
	f, err := os.Create("stamps.json")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	x, err := json.MarshalIndent(stamps.stamps, "", "  ")
	if err != nil {
		panic(err)
	}
	f.Write(x)
}

func readStamps() {
	f, err := os.ReadFile("stamps.json")
	if err != nil {
		if os.IsNotExist(err) {
			stamps = &stampList{}
			stamps.stamps = []timestamp{}
			stamps.Mutex = &sync.Mutex{}
			return
		}

		panic(err)
	}
	stamps = &stampList{}

	err = json.Unmarshal(f, &stamps.stamps)
	if err != nil {
		panic(err)
	}
	stamps.Mutex = &sync.Mutex{}
}
