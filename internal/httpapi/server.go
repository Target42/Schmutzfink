package httpapi

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"schmutzfink/internal/audit"
	"schmutzfink/internal/auth"
	"schmutzfink/internal/clip"
	"schmutzfink/internal/embedjob"
	"schmutzfink/internal/records"
	"schmutzfink/internal/storage"
	"schmutzfink/internal/webui"
)

type ctxKey int

const principalKey ctxKey = 1

type Server struct {
	Auth            *auth.Service
	Records         *records.Repo
	Store           storage.Store
	Clip            *clip.Engine
	Jobs            *embedjob.Worker
	WebDist         string
	CookieSecure    string
	TrustProxy      string
	ThumbCache      string
	Audit           *audit.Repo
	ProvisionToken  string
	ProvisionTenant string
	MobileAPKPath   string
	loginLimit      *loginLimiter
}

func (s *Server) Router() http.Handler {
	if s.loginLimit == nil {
		s.loginLimit = newLoginLimiter()
	}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(s.withClientAddr)
	r.Use(s.securityHeaders)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(2 * time.Minute))

	r.Get("/api/health", s.health)
	r.Post("/api/auth/login", s.login)
	r.Post("/api/auth/logout", s.logout)
	r.Group(func(r chi.Router) {
		r.Use(s.requireProvision)
		r.Post("/api/provision/users", s.provisionCreateUser)
		r.Get("/api/provision/users/{username}", s.provisionGetUser)
		r.Patch("/api/provision/users/{username}", s.provisionPatchUser)
	})

	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth)
		r.Get("/api/auth/me", s.me)
		r.Post("/api/auth/password", s.changePassword)
		r.Get("/api/tokens", s.listTokens)
		r.Post("/api/tokens", s.createToken)
		r.Delete("/api/tokens/{id}", s.revokeToken)
		r.Group(func(r chi.Router) {
			r.Use(s.requireAdmin)
			r.Get("/api/users", s.listUsers)
			r.Post("/api/users", s.createUser)
			r.Patch("/api/users/{id}", s.patchUser)
			r.Post("/api/users/{id}/password", s.resetUserPassword)
			r.Get("/api/audit", s.listAudit)
			r.Post("/api/fields", s.createField)
			r.Patch("/api/fields/{id}", s.patchField)
			r.Delete("/api/fields/{id}", s.deleteField)
		})
		r.Get("/api/fields", s.listFields)
		r.Group(func(r chi.Router) {
			r.Use(s.requireDeletionOfficer)
			r.Get("/api/deletion-requests", s.listDeletionRequests)
			r.Delete("/api/records/{id}", s.deleteRecord)
		})
		r.Get("/api/geocode", s.geocode)
		r.Get("/api/geocode/reverse", s.reverseGeocode)
		r.Get("/api/embeddings", s.embeddingsStatus)
		r.Get("/api/motifs", s.listMotifs)
		r.Get("/api/motifs/{id}", s.getMotif)
		r.Get("/api/motifs/{id}/suggestions", s.motifSuggestions)
		r.Get("/api/cases", s.listCases)
		r.Get("/api/cases/{id}", s.getCase)
		r.Get("/api/records", s.listRecords)
		r.Get("/api/records/map", s.mapRecords)
		r.Get("/api/records/export", s.exportRecords)
		r.Post("/api/records/export", s.exportRecords)
		r.Get("/api/records/summary", s.summaryRecords)
		r.Post("/api/records/summary", s.summaryRecords)
		r.Post("/api/search/image", s.searchByImage)
		r.Get("/api/records/{id}", s.getRecord)
		r.Get("/api/records/{id}/original", s.serveOriginal)
		r.Get("/api/records/{id}/thumb", s.serveThumb)
		r.Get("/api/sightings/{id}/similar", s.similarSightings)
		r.Get("/api/sightings/{id}/thumb", s.serveSightingThumb)
		r.Group(func(r chi.Router) {
			r.Use(s.requireWriter)
			r.Get("/api/mobile/android", s.mobileAndroidInfo)
			r.Get("/api/mobile/android.apk", s.downloadAndroidAPK)
			r.Post("/api/motifs", s.createMotif)
			r.Patch("/api/motifs/{id}", s.patchMotif)
			r.Delete("/api/motifs/{id}", s.deleteMotif)
			r.Post("/api/motifs/{id}/sightings", s.assignMotifSighting)
			r.Delete("/api/motifs/{id}/sightings/{sid}", s.unlinkMotifSighting)
			r.Post("/api/cases", s.createCase)
			r.Patch("/api/cases/{id}", s.patchCase)
			r.Delete("/api/cases/{id}", s.deleteCase)
			r.Post("/api/cases/{id}/records", s.assignCaseRecord)
			r.Delete("/api/cases/{id}/records/{rid}", s.unlinkCaseRecord)
			r.Post("/api/cases/{id}/close", s.closeCase)
			r.Post("/api/cases/{id}/reopen", s.reopenCase)
			r.Post("/api/records", s.upload)
			r.Post("/api/records/import", s.importArchive)
			r.Patch("/api/records/{id}", s.patchRecord)
			r.Post("/api/records/{id}/deletion-request", s.requestDeletion)
			r.Delete("/api/records/{id}/deletion-request", s.cancelDeletion)
			r.Post("/api/records/{id}/sightings", s.addSighting)
			r.Patch("/api/sightings/{id}", s.patchSighting)
			r.Delete("/api/sightings/{id}", s.deleteSighting)
		})
	})

	s.mountSPA(r)
	return r
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := s.lookupPrincipal(w, r)
		if !ok {
			return
		}
		if passwordChangeBlocked(r.URL.Path, p.MustChangePassword) {
			writeJSON(w, http.StatusForbidden, map[string]string{
				"error": "Passwort muss geändert werden",
				"code":  "must_change_password",
			})
			return
		}
		ctx := context.WithValue(r.Context(), principalKey, p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) lookupPrincipal(w http.ResponseWriter, r *http.Request) (auth.Principal, bool) {
	if raw, hasBearer := bearerToken(r); hasBearer {
		if raw == "" || s.Auth == nil {
			writeError(w, http.StatusUnauthorized, "Nicht angemeldet")
			return auth.Principal{}, false
		}
		p, err := s.Auth.LookupAPIToken(r.Context(), raw)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Nicht angemeldet")
			return auth.Principal{}, false
		}
		if !tokenPathAllowed(r.Method, r.URL.Path) {
			writeError(w, http.StatusForbidden, "API-Token darf nur lesen (Suche, Export, Metadaten)")
			return auth.Principal{}, false
		}
		return p, true
	}
	c, err := r.Cookie(auth.CookieName)
	if err != nil || c.Value == "" {
		writeError(w, http.StatusUnauthorized, "Nicht angemeldet")
		return auth.Principal{}, false
	}
	p, err := s.Auth.Lookup(r.Context(), c.Value)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Nicht angemeldet")
		return auth.Principal{}, false
	}
	return p, true
}

func principal(r *http.Request) auth.Principal {
	p, _ := r.Context().Value(principalKey).(auth.Principal)
	return p
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	out := map[string]any{"status": "ok"}
	if s.Clip != nil {
		out["embeddings"] = s.Clip.Status()
	}
	writeJSON(w, http.StatusOK, out)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) webFS() (fs.FS, bool) {
	// Lokales WEB_DIST hat Vorrang vor dem eingebetteten Build (siehe .env.example).
	if s.WebDist != "" {
		if _, err := os.Stat(s.WebDist); err == nil {
			return os.DirFS(s.WebDist), true
		}
	}
	if webui.Built() {
		fsys, err := webui.Files()
		return fsys, err == nil
	}
	fsys, err := webui.Files()
	return fsys, err == nil
}

func (s *Server) mountSPA(r chi.Router) {
	fsys, ok := s.webFS()
	if !ok {
		return
	}
	fileServer := http.FileServer(http.FS(fsys))
	r.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.URL.Path, "/api/") {
			http.NotFound(w, req)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+req.URL.Path), "/")
		if name == "" || name == "." {
			name = "index.html"
		}
		if f, err := fsys.Open(name); err == nil {
			info, statErr := f.Stat()
			_ = f.Close()
			if statErr == nil && !info.IsDir() {
				fileServer.ServeHTTP(w, req)
				return
			}
		}
		req = req.Clone(req.Context())
		req.URL.Path = "/"
		fileServer.ServeHTTP(w, req)
	}))
}
