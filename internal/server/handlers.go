package server

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/naftulisinger/datastar-template/internal/crypto"
	"github.com/naftulisinger/datastar-template/internal/db"
	"github.com/naftulisinger/datastar-template/internal/views"
	"github.com/starfederation/datastar-go/datastar"
)

// the *Page handlers return full html pages, everything else is called
// by datastar (@get, @post, ...) and answers with SSE patches instead

// -------- index
func (s *Server) indexPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		render(w, r, views.Shell(views.Index()))
	}
}

// -------- todos
func (s *Server) todosPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := s.DB.ListTodos(r.Context())

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		render(w, r, views.Shell(views.TodoPage(result)))
	}
}

func (s *Server) todosSearch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Read signals from the request
		todoSignals := &views.TodoPageSignals{}
		if err := datastar.ReadSignals(r, todoSignals); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		searchParam := strings.TrimSpace(todoSignals.SearchParam)

		result := []db.Todo{}
		var err error

		// empty search param means return all todos
		if searchParam == "" {
			result, err = s.DB.ListTodos(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			result, err = s.DB.SearchTodos(r.Context(), sql.NullString{String: searchParam, Valid: true})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		response, err := compToString(views.TodoList(result))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Create a Server-Sent Event writer
		sse := datastar.NewSSE(w, r)

		// TodoList renders with the same id as the list already on the page,
		// so datastar swaps it in place, no selector needed
		sse.PatchElements(response)
	}
}

func (s *Server) todosCreate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Read signals from the request
		todoSignals := &views.TodoPageSignals{}
		if err := datastar.ReadSignals(r, todoSignals); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		description := todoSignals.NewDescription

		if description == "" {
			http.Error(w, "Description cannot be empty", http.StatusBadRequest)
			return
		}

		newTodo, err := s.DB.CreateTodo(r.Context(), db.CreateTodoParams{
			Description: description,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		response, err := compToString(views.TodoItem(newTodo))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// the new todo isn't on the page yet, so tell datastar where it goes
		sse := datastar.NewSSE(w, r)
		sse.PatchElements(response,
			datastar.WithSelectorID("todo-list"),
			datastar.WithModePrepend(),
		)
	}
}

func (s *Server) todosUpdate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the todo ID from the route parameter.
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid todo ID", http.StatusBadRequest)
			return
		}

		// Get the action option
		action := r.PathValue("action")

		switch action {
		case "toggle":
			updatedToDo, err := s.DB.ToggleTodoCompleted(r.Context(), id)
			if err != nil {
				http.Error(w, "failed to toggle to do", http.StatusBadRequest)
				return
			}
			response, err := compToString(views.TodoItem(updatedToDo))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			sse := datastar.NewSSE(w, r)
			sse.PatchElements(response)

		case "update":
			// no signals here, the input lives in a <form> and the frontend
			// sends it as regular form data (contentType: 'form')
			description := strings.TrimSpace(r.FormValue("description"))

			if description == "" {
				http.Error(w, "Description cannot be empty", http.StatusBadRequest)
				return
			}
			updatedToDo, err := s.DB.UpdateTodoDescription(r.Context(), db.UpdateTodoDescriptionParams{
				Description: description,
				ID:          id,
			})
			if err != nil {
				http.Error(w, "failed to update to do", http.StatusBadRequest)
				return
			}
			response, err := compToString(views.TodoItem(updatedToDo))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			sse := datastar.NewSSE(w, r)
			sse.PatchElements(response)
			// prints in the browser devtools console, handy for debugging
			sse.ConsoleLog(fmt.Sprintf("Successfully updated task %v", updatedToDo.ID))

		default:
			http.Error(w, "invalid action", http.StatusBadRequest)
			return
		}
	}
}

func (s *Server) todosDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the todo ID from the route parameter.
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid todo ID", http.StatusBadRequest)
			return
		}
		err = s.DB.DeleteTodo(r.Context(), id)
		if err != nil {
			http.Error(w, "failed to delete to do", http.StatusBadRequest)
			return
		}

		// nothing to render, just remove the row
		sse := datastar.NewSSE(w, r)
		sse.RemoveElementByID("todo-" + idStr)
	}
}

// -------- crypto
func (s *Server) cryptoPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		render(w, r, views.Shell(views.CryptoPage(s.Crypto.Snapshot())))
	}
}

// unlike the other handlers this one never finishes on its own, the request
// stays open and keeps sending patches until the user leaves the page
func (s *Server) cryptoPrices() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		updates, unsubscribe := s.Crypto.Subscribe()
		defer unsubscribe()

		sse := datastar.NewSSE(w, r)

		// the feed can send many updates per second, so keep only the
		// latest ticker per product and flush them on an interval
		pending := map[string]crypto.Ticker{}
		flush := time.NewTicker(250 * time.Millisecond)
		defer flush.Stop()

		for {
			select {
			case <-r.Context().Done():
				return

			case t, ok := <-updates:
				if !ok {
					return
				}
				pending[t.ProductID] = t

			case <-flush.C:
				if len(pending) == 0 {
					continue
				}

				// each card has its own id, so datastar morphs them all from one patch
				var cards strings.Builder
				for product, t := range pending {
					card, err := compToString(views.PriceCard(product, t))
					if err != nil {
						sse.ConsoleError(err)
						return
					}
					cards.WriteString(card)
				}
				clear(pending)

				if err := sse.PatchElements(cards.String()); err != nil {
					return
				}
			}
		}
	}
}

// -------- healthcheck
func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
