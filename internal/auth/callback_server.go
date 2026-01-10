package auth

import (
    "net/http"
)

func StartCallbackServer(codeCh chan string) *http.Server {
    mux := http.NewServeMux()

    mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
        code := r.URL.Query().Get("code")
        if code != "" {
            codeCh <- code
            w.Write([]byte("Login successful. You can close this window."))
        }
    })

    srv := &http.Server{
        Addr:    ":8085",
        Handler: mux,
    }

    go srv.ListenAndServe()

    return srv
}
