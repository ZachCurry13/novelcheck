package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Kindle tasks (Library cleanup → Kindle tasks): things the admin asks for in the app that a Kindle does at its next
// sync: back up, tidy the books, delete crash logs, delete books (only ones NovelCheck holds a copy of), send library
// books, move books into a folder. The plugin asks for the waiting ones (GET /kosync/plugin/tasks), does them, and
// reports each (POST /kosync/plugin/tasks/result). Only these kinds exist; nothing else is ever run.

// kindleBackedUp says whether the server holds a copy of a file with this name and size anywhere in a Kindle's copy.
func (s *Server) kindleBackedUp(userID int64, device string) map[string]bool {
	have := map[string]bool{} // "name|size"
	root := s.kindleDir(userID, device)
	_ = filepath.WalkDir(root, func(p string, e os.DirEntry, err error) error {
		if err != nil || e.IsDir() || strings.HasSuffix(p, ".part") {
			return nil
		}
		if fi, err := e.Info(); err == nil {
			have[e.Name()+"|"+strconv.FormatInt(fi.Size(), 10)] = true
		}
		return nil
	})
	return have
}

// handleKindleTasks: GET /api/admin/kindle-tasks?device= lists the Kindles, the newest tasks and (for the chosen
// Kindle) its book files with whether the server holds a copy; POST creates a task; DELETE /{id} takes one back.
func (s *Server) handleKindleTasks(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	if r.Method == http.MethodGet {
		devices := s.Store.Devices(u.ID)
		device := r.URL.Query().Get("device")
		if device == "" && len(devices) > 0 {
			device = devices[0].Name
		}
		type fileRow struct {
			Filename string `json:"filename"`
			Title    string `json:"title"`
			Author   string `json:"author"`
			Size     int64  `json:"size"`
			BackedUp bool   `json:"backed_up"`
		}
		files := []fileRow{}
		if device != "" {
			have := s.kindleBackedUp(u.ID, device)
			for _, b := range s.Store.DeviceFiles(u.ID, device) {
				files = append(files, fileRow{b.Filename, b.Title, b.Author, b.Size, have[b.Filename+"|"+strconv.FormatInt(b.Size, 10)]})
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"devices": devices, "device": device, "files": files, "tasks": s.Store.KindleTasksOf(u.ID, 30)})
		return
	}
	var body struct {
		Device    string   `json:"device"`
		Kind      string   `json:"kind"`
		Filenames []string `json:"filenames"` // delete_books, move_books
		Folder    string   `json:"folder"`    // move_books
		BookIDs   []int64  `json:"book_ids"`  // download_books
		Switch    string   `json:"switch"`    // set_auto: one of store.KindleAutoSwitches
		On        bool     `json:"on"`        // set_auto
	}
	if !readJSON(w, r, &body, 256<<10) {
		return
	}
	if !store.KindleTaskKinds[body.Kind] {
		writeErr(w, http.StatusBadRequest, "that isn't something a Kindle can be asked to do")
		return
	}
	args := map[string]any{}
	switch body.Kind {
	case "delete_books", "move_books":
		device := body.Device
		if device == "" {
			writeErr(w, http.StatusBadRequest, "pick a Kindle first")
			return
		}
		have := s.kindleBackedUp(u.ID, device)
		sizes := map[string]int64{}
		for _, f := range s.Store.DeviceFiles(u.ID, device) {
			sizes[f.Filename] = f.Size
		}
		var items []map[string]any
		var notSafe []string
		for _, name := range body.Filenames {
			size, ok := sizes[name]
			if !ok {
				continue
			}
			if !have[name+"|"+strconv.FormatInt(size, 10)] {
				notSafe = append(notSafe, name)
				continue
			}
			items = append(items, map[string]any{"filename": name, "size": size})
		}
		if len(notSafe) > 0 {
			writeErr(w, http.StatusConflict, "NovelCheck holds no backup copy of: "+strings.Join(notSafe[:min(len(notSafe), 5)], ", ")+". Back the Kindle up first, then try again. Nothing was queued.")
			return
		}
		if len(items) == 0 {
			writeErr(w, http.StatusBadRequest, "tick at least one book on that Kindle")
			return
		}
		args["items"] = items
		if body.Kind == "move_books" {
			folder := strings.TrimSpace(strings.Map(func(r rune) rune {
				if r == '/' || r == '\\' || r == ':' || r < 32 {
					return ' '
				}
				return r
			}, body.Folder))
			if folder == "" || folder == "." || folder == ".." || len(folder) > 60 {
				writeErr(w, http.StatusBadRequest, "give a folder name (one folder under Books)")
				return
			}
			args["folder"] = folder
		}
	case "set_auto":
		if !store.KindleAutoSwitches[body.Switch] {
			writeErr(w, http.StatusBadRequest, "that isn't one of the plugin's automatic switches")
			return
		}
		args["switch"], args["on"] = body.Switch, body.On
	case "download_books":
		var books []pluginBook
		for _, id := range body.BookIDs {
			if b, err := s.Store.BookByID(id, u); err == nil {
				if pb, ok := s.pluginBookFor(b); ok {
					books = append(books, pb)
				}
			}
		}
		if len(books) == 0 {
			writeErr(w, http.StatusBadRequest, "none of those books has a file NovelCheck can send")
			return
		}
		args["books"] = books
	}
	raw, _ := json.Marshal(args)
	id := s.Store.CreateKindleTask(u.ID, strings.TrimSpace(body.Device), body.Kind, string(raw))
	if id == 0 {
		writeErr(w, http.StatusInternalServerError, "couldn't save the task")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"id": id})
}

// handleKindleTaskCancel: DELETE /api/admin/kindle-tasks/{id}.
func (s *Server) handleKindleTaskCancel(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if !s.Store.CancelKindleTask(auth.UserFrom(r).ID, id) {
		writeErr(w, http.StatusNotFound, "that task was already done or taken back")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handlePluginTasks: GET /kosync/plugin/tasks?device= gives the Kindle its waiting tasks.
func (s *Server) handlePluginTasks(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	type task struct {
		ID   int64           `json:"id"`
		Kind string          `json:"kind"`
		Args json.RawMessage `json:"args"`
	}
	out := []task{}
	for _, t := range s.Store.WaitingKindleTasks(u.ID, strings.TrimSpace(r.URL.Query().Get("device"))) {
		out = append(out, task{t.ID, t.Kind, json.RawMessage(t.Args)})
	}
	kosyncJSON(w, http.StatusOK, map[string]any{"tasks": out})
}

// handlePluginTaskResult: POST /kosync/plugin/tasks/result {"id", "status": done|failed|skipped, "note"}.
func (s *Server) handlePluginTaskResult(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	var body struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&body); err != nil || body.ID <= 0 {
		kosyncJSON(w, http.StatusBadRequest, map[string]string{"message": "bad request"})
		return
	}
	kosyncJSON(w, http.StatusOK, map[string]bool{"ok": s.Store.SetKindleTaskResult(u.ID, body.ID, body.Status, body.Note)})
}
