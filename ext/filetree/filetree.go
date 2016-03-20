package filetree

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"

	"github.com/gorilla/mux"
	log "gopkg.in/inconshreveable/log15.v2"

	"github.com/tsileo/blobstash/embed"
	"github.com/tsileo/blobstash/ext/filetree/filetreeutil/meta"
	"github.com/tsileo/blobstash/ext/filetree/reader/filereader"
	"github.com/tsileo/blobstash/httputil"
	serverMiddleware "github.com/tsileo/blobstash/middleware"
	_ "github.com/tsileo/blobstash/permissions"
	_ "github.com/tsileo/blobstash/vkv"
)

// TODO(tsileo): handle the fetching of meta from the FS name and reconstruct the vkkeky, also ensure XAttrs are public and keep a ref
// to the root in children link

var (
	indexFile = "index.html"

	PermName     = "filetree"
	PermTreeName = "filetree:root:"
	PermWrite    = "write"
	PermRead     = "read"
)

type FileTreeExt struct {
	kvStore   *embed.KvStore
	blobStore *embed.BlobStore

	log log.Logger
}

// New initializes the `DocStoreExt`
func New(logger log.Logger, kvStore *embed.KvStore, blobStore *embed.BlobStore) (*FileTreeExt, error) {
	return &FileTreeExt{
		kvStore:   kvStore,
		blobStore: blobStore,
		log:       logger,
	}, nil
}

// Close closes all the open DB files.
func (ft *FileTreeExt) Close() error {
	return nil
}

// RegisterRoute registers all the HTTP handlers for the extension
func (ft *FileTreeExt) RegisterRoute(root, r *mux.Router, middlewares *serverMiddleware.SharedMiddleware) {
	// FIXME(tsileo): also bind to the root router to allow {hostname}/f/{ref} and {hostname}/d/{ref} to share file/dir
	ft.log.Debug("RegisterRoute")
	dirHandler := http.HandlerFunc(ft.dirHandler())
	fileHandler := http.HandlerFunc(ft.fileHandler())
	r.Handle("/node/{ref}", middlewares.Auth(http.HandlerFunc(ft.nodeHandler())))
	r.Handle("/dir/{ref}", dirHandler)
	r.Handle("/file/{ref}", fileHandler)
	// Enable shortcut path from the root
	root.Handle("/d/{ref}", dirHandler)
	root.Handle("/f/{ref}", fileHandler)

	// r.Handle("/file/{ref}", middlewares.Auth(http.HandlerFunc(ft.fileHandler())))
	// r.Handle("/", middlewares.Auth(http.HandlerFunc(docstore.collectionsHandler())))
}

type Node struct {
	// Version string                 `json:"version"`

	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	Size     int                    `json:"size"`
	Mode     uint32                 `json:"mode"`
	ModTime  string                 `json:"mtime"`
	Extra    map[string]interface{} `json:"extra,omitempty"`
	Hash     string                 `json:"ref"`
	Children []*Node                `json:"children,omitempty"`

	meta *meta.Meta `json:"-"`
}

func (n *Node) Close() error {
	n.meta.Close()
	return nil
}

type byName []*Node

func (s byName) Len() int           { return len(s) }
func (s byName) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s byName) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func metaToNode(m *meta.Meta) (*Node, error) {
	return &Node{
		Name:    m.Name,
		Type:    m.Type,
		Size:    m.Size,
		Mode:    m.Mode,
		ModTime: m.ModTime,
		Extra:   m.Extra,
		Hash:    m.Hash,
		meta:    m,
	}, nil
}

func (ft *FileTreeExt) fetchDir(n *Node, depth int) error {
	if depth >= 10 {
		return nil
	}
	if n.Type == "dir" {
		n.Children = []*Node{}
		for _, ref := range n.meta.Refs {
			cn, err := ft.nodeByRef(ref.(string))
			if err != nil {
				return err
			}
			n.Children = append(n.Children, cn)
			if err := ft.fetchDir(cn, depth+1); err != nil {
				return err
			}
		}
	}
	n.meta.Close()
	return nil
}

func (ft *FileTreeExt) fileHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// permissions.CheckPerms(r, PermName)

		if r.Method != "GET" && r.Method != "HEAD" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		vars := mux.Vars(r)

		hash := vars["ref"]
		ft.serveFile(w, r, hash)
	}
}

func (ft *FileTreeExt) serveFile(w http.ResponseWriter, r *http.Request, hash string) {
	blob, err := ft.blobStore.Get(hash)
	if err != nil {
		panic(err)
	}

	m, err := meta.NewMetaFromBlob(hash, blob)
	if err != nil {
		panic(err)
	}
	defer m.Close()

	// Initialize a new `File`
	f := filereader.NewFile(ft.blobStore, m)

	// Check if the file is requested for download
	if r.URL.Query().Get("dl") != "" {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", m.Name))
	}

	// Serve the file content using the same code as the `http.ServeFile`
	mtime, _ := m.Mtime()
	http.ServeContent(w, r, m.Name, mtime, f)
}

func (ft *FileTreeExt) nodeHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// permissions.CheckPerms(r, PermName)

		// TODO(tsileo): limit the max depth of the tree
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		// TODO(tsileo): handle HEAD request and returns a 404 if not exsit, same for /fileHandler

		vars := mux.Vars(r)

		hash := vars["ref"]
		n, err := ft.nodeByRef(hash)
		if err != nil {
			panic(err)
		}
		if err := ft.fetchDir(n, 1); err != nil {
			panic(err)
		}

		httputil.WriteJSON(w, map[string]interface{}{
			"root": n,
		})
	}
}

func (ft *FileTreeExt) nodeByRef(hash string) (*Node, error) {
	blob, err := ft.blobStore.Get(hash)
	if err != nil {
		return nil, err
	}

	m, err := meta.NewMetaFromBlob(hash, blob)
	if err != nil {
		return nil, err
	}

	n, err := metaToNode(m)
	if err != nil {
		return nil, err
	}

	return n, nil
}

func (ft *FileTreeExt) dirHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		vars := mux.Vars(r)

		hash := vars["ref"]
		n, err := ft.nodeByRef(hash)
		if err != nil {
			panic(err)
		}
		if err := ft.fetchDir(n, 1); err != nil {
			panic(err)
		}

		sort.Sort(byName(n.Children))

		// Check if the dir contains an "index.html")
		for _, cn := range n.Children {
			if cn.Name == indexFile {
				ft.serveFile(w, r, cn.Hash)
				return
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<!doctype html><title>Filetree - %s</title><pre>\n")
		for _, cn := range n.Children {
			var curl url.URL
			curl = url.URL{Path: fmt.Sprintf("/api/ext/filetree/v1/%s/%s", cn.Type, cn.Hash)}
			fmt.Fprintf(w, "<a href=\"%s\">%s</a>\n", curl.String(), cn.Name)
		}
		fmt.Fprintf(w, "</pre>\n")
	}
}
