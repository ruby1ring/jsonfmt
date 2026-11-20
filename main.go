// jsonfmt：本机 JSON 格式化工具。
// 页面（index.html + codemirror.js）编进二进制，历史记录存在 ~/.jsonfmt/history.json。
package main

import (
	"crypto/rand"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed index.html
var indexHTML []byte

// web/ 目录打包出来的 CodeMirror 编辑器，改动方式见 web/package.json
//
//go:embed codemirror.js
var editorJS []byte

// 用户确认过：历史只保留最近 200 条，超出时删掉最旧的
const maxHistory = 200

type Entry struct {
	ID      string `json:"id"`
	Time    int64  `json:"time"` // unix 毫秒
	Content string `json:"content"`
}

type Summary struct {
	ID      string `json:"id"`
	Time    int64  `json:"time"`
	Size    int    `json:"size"`
	Preview string `json:"preview"`
}

var (
	historyLock sync.Mutex
	history     []Entry // 最新的在最前面
	historyPath string
)

func main() {
	address := flag.String("addr", "127.0.0.1:7788", "监听地址")
	flag.Parse()

	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("get home dir: %v", err)
	}
	dataDir := filepath.Join(home, ".jsonfmt")
	// 历史里可能有接口返回的敏感数据，目录和文件只给自己读写
	err = os.MkdirAll(dataDir, 0o700)
	if err != nil {
		log.Fatalf("create %s: %v", dataDir, err)
	}
	historyPath = filepath.Join(dataDir, "history.json")

	data, err := os.ReadFile(historyPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Fatalf("read %s: %v", historyPath, err)
	}
	if err == nil {
		err = json.Unmarshal(data, &history)
		if err != nil {
			log.Fatalf("parse %s: %v", historyPath, err)
		}
	}

	http.HandleFunc("GET /{$}", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.Write(indexHTML)
	})

	http.HandleFunc("GET /codemirror.js", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		writer.Write(editorJS)
	})

	// 列表只回传预览，完整内容点开某一条时再按 id 取
	http.HandleFunc("GET /api/history", func(writer http.ResponseWriter, request *http.Request) {
		query := strings.ToLower(request.URL.Query().Get("q"))

		historyLock.Lock()
		list := []Summary{}
		for _, entry := range history {
			if query != "" && !strings.Contains(strings.ToLower(entry.Content), query) {
				continue
			}
			// 预览只用于列表展示，取前 300 字节压掉空白；切断的半个 UTF-8 字符直接去掉
			preview := entry.Content
			if len(preview) > 300 {
				preview = preview[:300]
			}
			preview = strings.Join(strings.Fields(strings.ToValidUTF8(preview, "")), " ")
			list = append(list, Summary{ID: entry.ID, Time: entry.Time, Size: len(entry.Content), Preview: preview})
		}
		historyLock.Unlock()

		writeJSON(writer, list)
	})

	http.HandleFunc("GET /api/history/{id}", func(writer http.ResponseWriter, request *http.Request) {
		id := request.PathValue("id")
		historyLock.Lock()
		defer historyLock.Unlock()
		for _, entry := range history {
			if entry.ID == id {
				writeJSON(writer, entry)
				return
			}
		}
		http.Error(writer, "history not found: "+id, http.StatusNotFound)
	})

	// 新建一条历史。内容和已有某条完全相同时，沿用那条的 id 并挪到最前面
	http.HandleFunc("POST /api/history", func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			http.Error(writer, "read body: "+err.Error(), http.StatusBadRequest)
			return
		}
		content := string(body)
		if strings.TrimSpace(content) == "" {
			http.Error(writer, "empty content", http.StatusBadRequest)
			return
		}

		historyLock.Lock()
		defer historyLock.Unlock()
		id := rand.Text()
		for _, entry := range history {
			if entry.Content == content {
				id = entry.ID
				break
			}
		}
		err = putOnTop(Entry{ID: id, Time: time.Now().UnixMilli(), Content: content})
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, map[string]string{"id": id})
	})

	// 页面上持续编辑同一份内容时，原地更新那一条，而不是每次都新增
	http.HandleFunc("PUT /api/history/{id}", func(writer http.ResponseWriter, request *http.Request) {
		id := request.PathValue("id")
		body, err := io.ReadAll(request.Body)
		if err != nil {
			http.Error(writer, "read body: "+err.Error(), http.StatusBadRequest)
			return
		}
		content := string(body)
		if strings.TrimSpace(content) == "" {
			http.Error(writer, "empty content", http.StatusBadRequest)
			return
		}

		historyLock.Lock()
		defer historyLock.Unlock()
		found := false
		for _, entry := range history {
			if entry.ID == id {
				found = true
				break
			}
		}
		// 这条已经被删掉（比如在历史面板里删了），由页面改为新建
		if !found {
			http.Error(writer, "history not found: "+id, http.StatusNotFound)
			return
		}
		err = putOnTop(Entry{ID: id, Time: time.Now().UnixMilli(), Content: content})
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, map[string]string{"id": id})
	})

	http.HandleFunc("DELETE /api/history/{id}", func(writer http.ResponseWriter, request *http.Request) {
		id := request.PathValue("id")
		historyLock.Lock()
		defer historyLock.Unlock()
		next := []Entry{}
		for _, entry := range history {
			if entry.ID != id {
				next = append(next, entry)
			}
		}
		history = next
		err := saveHistory()
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	})

	http.HandleFunc("DELETE /api/history", func(writer http.ResponseWriter, request *http.Request) {
		historyLock.Lock()
		defer historyLock.Unlock()
		history = []Entry{}
		err := saveHistory()
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	})

	log.Printf("jsonfmt listening on http://%s , history file %s", *address, historyPath)
	log.Fatal(http.ListenAndServe(*address, nil))
}

// 调用方必须持有 historyLock。
// 把 entry 放到最前面，去掉 id 相同或内容相同的旧记录，超出上限删最旧的，然后落盘。
func putOnTop(entry Entry) error {
	next := []Entry{entry}
	for _, old := range history {
		if old.ID != entry.ID && old.Content != entry.Content {
			next = append(next, old)
		}
	}
	if len(next) > maxHistory {
		next = next[:maxHistory]
	}
	history = next
	return saveHistory()
}

// 调用方必须持有 historyLock。
// 先写临时文件再 rename，进程中途退出也不会把历史文件写坏。
func saveHistory() error {
	data, err := json.Marshal(history)
	if err != nil {
		return fmt.Errorf("encode history: %w", err)
	}
	tempPath := historyPath + ".tmp"
	err = os.WriteFile(tempPath, data, 0o600)
	if err != nil {
		return fmt.Errorf("write %s: %w", tempPath, err)
	}
	err = os.Rename(tempPath, historyPath)
	if err != nil {
		return fmt.Errorf("rename %s: %w", tempPath, err)
	}
	return nil
}

func writeJSON(writer http.ResponseWriter, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	err := json.NewEncoder(writer).Encode(value)
	if err != nil {
		log.Printf("write response: %v", err)
	}
}
