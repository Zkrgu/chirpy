package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/zkrgu/chirpy/internal/auth"
	"github.com/zkrgu/chirpy/internal/database"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	jwtSecret      string
	polkaKey       string
}

func main() {
	godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	jwtSecret := os.Getenv("JWT_SECRET")
	polkaKey := os.Getenv("POLKA_KEY")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	// ignore unused
	dbQueries := database.New(db)
	_ = dbQueries
	apiCfg := apiConfig{
		db:        dbQueries,
		jwtSecret: jwtSecret,
		polkaKey:  polkaKey,
	}

	sm := http.NewServeMux()

	sm.HandleFunc("GET /api/healthz", healthHandler)
	sm.HandleFunc("POST /api/users", apiCfg.createUserHandler)
	sm.HandleFunc("PUT /api/users", apiCfg.updateUserHandler)
	sm.HandleFunc("POST /api/login", apiCfg.loginHandler)
	sm.HandleFunc("POST /api/refresh", apiCfg.refreshHandler)
	sm.HandleFunc("POST /api/revoke", apiCfg.revokeHandler)
	sm.HandleFunc("GET /api/chirps", apiCfg.getChirpsHandler)
	sm.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.getChirpHandler)
	sm.HandleFunc("DELETE /api/chirps/{chirpID}", apiCfg.deleteChirpHandler)
	sm.HandleFunc("POST /api/chirps", apiCfg.validateChirpHandler)
	sm.HandleFunc("POST /api/polka/webhooks", apiCfg.webhookHandler)
	sm.HandleFunc("GET /admin/metrics", apiCfg.metricsHandler)
	sm.HandleFunc("POST /admin/reset", apiCfg.resetHandler)
	sm.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app/", http.FileServer(http.Dir('.')))))

	server := http.Server{
		Addr:    ":8080",
		Handler: sm,
	}
	log.Fatal(server.ListenAndServe())
}

func healthHandler(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Add("Content-Type", "text/plain; charset=utf-8")
	rw.WriteHeader(200)
	rw.Write([]byte("OK"))
}

const metricsTemplate = `<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`

func (cfg *apiConfig) metricsHandler(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Add("Content-Type", "text/html")
	rw.WriteHeader(200)
	rw.Write([]byte(fmt.Sprintf(metricsTemplate, cfg.fileserverHits.Load())))
}

func (cfg *apiConfig) resetHandler(rw http.ResponseWriter, req *http.Request) {
	cfg.fileserverHits.Store(0)
	cfg.db.DeleteUsers(req.Context())
	rw.WriteHeader(200)
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

// type chirpValid struct {
// 	Body   string    `json:"body"`
// 	UserId uuid.UUID `json:"user_id"`
// }

func (cfg *apiConfig) validateChirpHandler(rw http.ResponseWriter, req *http.Request) {
	token, err := auth.GetBearerToken(req.Header)
	if err != nil {
		rw.WriteHeader(401)
		return
	}
	id, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		rw.WriteHeader(401)
		return
	}

	var data database.CreateChirpParams
	decoder := json.NewDecoder(req.Body)
	err = decoder.Decode(&data)

	data.UserID = id

	if err != nil {
		rw.WriteHeader(500)
		fmt.Println(err)
		rw.Write([]byte(`{"error":"Something went wrong"}`))
		return
	}

	if len(data.Body) > 140 {
		rw.WriteHeader(400)
		rw.Write([]byte(`{"error":"Chirp is too long"}`))
		return
	}

	data.Body = replaceWords(data.Body, []string{"kerfuffle", "sharbert", "fornax"})

	chirp, err := cfg.db.CreateChirp(req.Context(), data)

	rw.WriteHeader(201)
	json.NewEncoder(rw).Encode(chirp)
}

func replaceWords(str string, bad []string) string {
	new_words := make([]string, 0)
	for _, word := range strings.Fields(str) {
		if slices.Contains(bad, strings.ToLower(word)) {
			new_words = append(new_words, "****")
		} else {
			new_words = append(new_words, word)
		}
	}
	return strings.Join(new_words, " ")
}

type createUser struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (cfg *apiConfig) createUserHandler(rw http.ResponseWriter, req *http.Request) {
	var params createUser
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&params)
	if err != nil {
		rw.WriteHeader(500)
		return
	}
	hp, err := auth.HashPassword(params.Password)
	user, err := cfg.db.CreateUser(req.Context(), database.CreateUserParams{
		Email:          params.Email,
		HashedPassword: hp,
	})
	if err != nil {
		fmt.Println(err)
		rw.WriteHeader(401)
		return
	}

	rw.WriteHeader(201)
	json.NewEncoder(rw).Encode(user)
}

func (cfg *apiConfig) getChirpsHandler(rw http.ResponseWriter, req *http.Request) {
	author := req.URL.Query().Get("author_id")
	sortParam := req.URL.Query().Get("sort")
	if len(sortParam) == 0 {
		sortParam = "asc"
	}
	aid, err := uuid.Parse(author)
	if len(author) > 0 && err != nil {
		rw.WriteHeader(404)
		return
	}
	var chirps []database.Chirp
	if aid == uuid.Nil {
		chirps, err = cfg.db.GetChirps(req.Context())
	} else {
		chirps, err = cfg.db.GetUserChirps(req.Context(), aid)
	}
	if err != nil {
		fmt.Println(err)
		rw.WriteHeader(500)
		return
	}

	if sortParam == "desc" {
		slices.Reverse(chirps)
	}
	json.NewEncoder(rw).Encode(chirps)
}

func (cfg *apiConfig) getChirpHandler(rw http.ResponseWriter, req *http.Request) {
	uid, err := uuid.Parse(req.PathValue("chirpID"))
	if err != nil {
		fmt.Println(err)
		rw.WriteHeader(500)
		return
	}
	chirps, err := cfg.db.GetChirp(req.Context(), uid)
	if err != nil {
		rw.WriteHeader(404)
		return
	}

	json.NewEncoder(rw).Encode(chirps)
}

type loginUser struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	// ExpiresInSeconds int    `json:"expires_in_seconds"`
}

type loginResponse struct {
	ID           uuid.UUID `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Email        string    `json:"email"`
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
	IsChirpyRed  bool      `json:"is_chirpy_red"`
}

func (cfg *apiConfig) loginHandler(rw http.ResponseWriter, req *http.Request) {
	var params loginUser
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&params)
	if err != nil {
		rw.WriteHeader(500)
		return
	}

	user, err := cfg.db.GetUserByEmail(req.Context(), params.Email)
	err = auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if err != nil {
		rw.WriteHeader(401)
		return
	}

	token, err := auth.MakeJWT(user.ID, cfg.jwtSecret, time.Duration(1)*time.Hour)

	if err != nil {
		rw.WriteHeader(500)
		return
	}

	refresh, err := auth.MakeRefreshToken()
	if err != nil {
		rw.WriteHeader(500)
		return
	}

	_, err = cfg.db.CreateRefreshToken(req.Context(), database.CreateRefreshTokenParams{
		Token:     refresh,
		UserID:    user.ID,
		ExpiresAt: time.Now().AddDate(0, 0, 60),
	})

	lr := loginResponse{
		ID:           user.ID,
		CreatedAt:    user.CreatedAt,
		Email:        user.Email,
		Token:        token,
		RefreshToken: refresh,
		IsChirpyRed:  user.IsChirpyRed,
	}

	json.NewEncoder(rw).Encode(lr)
}

type refreshResponse struct {
	Token string `json:"token"`
}

func (cfg *apiConfig) refreshHandler(rw http.ResponseWriter, req *http.Request) {
	token, err := auth.GetBearerToken(req.Header)
	if err != nil {
		rw.WriteHeader(401)
		return
	}
	rt, err := cfg.db.GetToken(req.Context(), token)
	if err != nil {
		rw.WriteHeader(401)
		return
	}
	at, err := auth.MakeJWT(rt.UserID, cfg.jwtSecret, time.Duration(1)*time.Hour)
	if err != nil {
		rw.WriteHeader(500)
		return
	}

	json.NewEncoder(rw).Encode(refreshResponse{Token: at})
}

func (cfg *apiConfig) revokeHandler(rw http.ResponseWriter, req *http.Request) {
	token, err := auth.GetBearerToken(req.Header)
	if err != nil {
		rw.WriteHeader(401)
		return
	}
	_, err = cfg.db.GetToken(req.Context(), token)
	if err != nil {
		rw.WriteHeader(401)
		return
	}
	err = cfg.db.RevokeToken(req.Context(), token)
	if err != nil {
		rw.WriteHeader(500)
		return
	}

	rw.WriteHeader(204)
}

func (cfg *apiConfig) updateUserHandler(rw http.ResponseWriter, req *http.Request) {
	token, err := auth.GetBearerToken(req.Header)
	if err != nil {
		rw.WriteHeader(401)
		return
	}
	id, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		rw.WriteHeader(401)
		return
	}
	var params createUser
	decoder := json.NewDecoder(req.Body)
	err = decoder.Decode(&params)
	if err != nil {
		rw.WriteHeader(500)
		return
	}
	hp, err := auth.HashPassword(params.Password)
	if err != nil {
		fmt.Println(err)
		rw.WriteHeader(500)
		return
	}
	user, err := cfg.db.UpdateUser(req.Context(), database.UpdateUserParams{
		ID:             id,
		Email:          params.Email,
		HashedPassword: hp,
	})
	if err != nil {
		fmt.Println(err)
		rw.WriteHeader(500)
		return
	}

	json.NewEncoder(rw).Encode(user)
}

func (cfg *apiConfig) deleteChirpHandler(rw http.ResponseWriter, req *http.Request) {
	token, err := auth.GetBearerToken(req.Header)
	if err != nil {
		rw.WriteHeader(401)
		return
	}
	uid, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		rw.WriteHeader(401)
		return
	}
	cid, err := uuid.Parse(req.PathValue("chirpID"))
	if err != nil {
		rw.WriteHeader(404)
		return
	}
	chirps, err := cfg.db.GetChirp(req.Context(), cid)
	if err != nil {
		rw.WriteHeader(404)
		return
	}
	if chirps.UserID != uid {
		rw.WriteHeader(403)
		return
	}
	err = cfg.db.DeleteChirp(req.Context(), cid)
	if err != nil {
		fmt.Println(err)
		rw.WriteHeader(500)
		return
	}
	rw.WriteHeader(204)
}

type polkaWebhook struct {
	Event string `json:"event"`
	Data  struct {
		UserId string `json:"user_id"`
	} `json:"data"`
}

func (cfg *apiConfig) webhookHandler(rw http.ResponseWriter, req *http.Request) {
	apiKey, err := auth.GetApiKey(req.Header)
	if err != nil || apiKey != cfg.polkaKey {
		rw.WriteHeader(401)
		return
	}
	var params polkaWebhook
	decoder := json.NewDecoder(req.Body)
	err = decoder.Decode(&params)
	if err != nil {
		rw.WriteHeader(400)
		return
	}
	uid, err := uuid.Parse(params.Data.UserId)
	if err != nil {
		rw.WriteHeader(400)
		return
	}
	if params.Event == "user.upgraded" {
		_, err = cfg.db.UpgradeUser(req.Context(), database.UpgradeUserParams{
			IsChirpyRed: true,
			ID:          uid,
		})
		if err != nil {
			rw.WriteHeader(404)
			return
		}
	}
	rw.WriteHeader(204)
}
