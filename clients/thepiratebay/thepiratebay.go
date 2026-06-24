package thepiratebay

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	neturl "net/url"
	"strconv"
	"time"

	"github.com/ismaelpadilla/gotorrent/interfaces"
	"github.com/skratchdot/open-golang/open"
)

func New() interfaces.Client {
	return pirateBay{}
}

func (p pirateBay) Search(a string) []interfaces.Torrent {
	// QueryEscape so spaces (and other unsafe chars) don't corrupt the
	// request line. Without this, `daft punk` becomes the malformed
	// "GET /q.php?q=daft punk HTTP/1.1" and apibay returns nothing.
	url := "https://apibay.org/q.php?q=" + neturl.QueryEscape(a)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Panic(err)
	}
	// apibay's edge returns an empty body to UA-less clients
	req.Header.Set("User-Agent", "gotorrent")

	client := &http.Client{Timeout: 15 * time.Second}
	result, err := client.Do(req)
	if err != nil {
		log.Panic(err)
	}
	defer result.Body.Close()

	body, err := io.ReadAll(result.Body)
	if err != nil {
		log.Panic(err)
	}

	// Guard: an empty or non-JSON body means the request was blocked,
	// redirected, or rate-limited. Return no results instead of crashing
	// the whole TUI with "unexpected end of JSON input".
	body = bytes.TrimSpace(body)
	if len(body) == 0 || body[0] != '[' {
		return []interfaces.Torrent{}
	}

	var bodyParsed []pirateBayTorrent
	if err := json.Unmarshal(body, &bodyParsed); err != nil {
		return []interfaces.Torrent{}
	}

	torrents := make([]interfaces.Torrent, len(bodyParsed))
	for i, pbt := range bodyParsed {
		torrents[i] = pbt.convert()
		torrents[i].Client = p
	}
	return torrents
}

func (p pirateBayTorrent) convert() interfaces.Torrent {
	magnetLink := "magnet:?xt=urn:btih:" + p.InfoHash
	size, err := strconv.Atoi(p.Size)
	if err != nil {
		log.Panic(err)
	}
	seeders, err := strconv.Atoi(p.Seeders)
	if err != nil {
		log.Panic(err)
	}
	leechers, err := strconv.Atoi(p.Leechers)
	if err != nil {
		log.Panic(err)
	}

	return interfaces.Torrent{
		ID:         p.ID,
		Title:      p.Name,
		InfoHash:   p.InfoHash,
		MagnetLink: magnetLink,
		Size:       size,
		Uploaded:   p.Added,
		Seeders:    seeders,
		Leechers:   leechers,
	}
}

func (p pirateBay) NavigateTo(torrent interfaces.Torrent) {
	url := p.getProxy() + "/description.php?id=" + torrent.ID
	err := open.Run(url)
	if err != nil {
		log.Panic(err)
	}
}

func (p pirateBay) FetchTorrentDescription(torrent interfaces.Torrent) string {
	url := "https://apibay.org/t.php?id=" + torrent.ID
	result, err := http.Get(url)
	if err != nil {
		log.Panic(err)
	}
	defer result.Body.Close()

	body, err := io.ReadAll(result.Body)
	if err != nil {
		log.Panic(err)
	}
	var bodyParsed pirateBayTorrentDetails
	err = json.Unmarshal(body, &bodyParsed)
	if err != nil {
		log.Panic("cant unmarshall", err)
	}

	return bodyParsed.Descr
}

func (p pirateBay) FetchTorrentFiles(torrent interfaces.Torrent) []interfaces.TorrentFile {
	url := "https://apibay.org/f.php?id=" + torrent.ID
	result, err := http.Get(url)
	if err != nil {
		log.Panic(err)
	}
	defer result.Body.Close()

	body, err := io.ReadAll(result.Body)
	if err != nil {
		log.Panic(err)
	}
	var bodyParsed []pirateBayTorrentFile
	err = json.Unmarshal(body, &bodyParsed)
	if err != nil {
		log.Panic("cant unmarshall", err)
	}

	torrentFiles := make([]interfaces.TorrentFile, len(bodyParsed))
	for i, pbtf := range bodyParsed {
		torrentFiles[i] = interfaces.TorrentFile{
			Name: pbtf.Name[0],
			Size: pbtf.Size[0],
		}
	}
	return torrentFiles
}

// get a valid proxy
func (p pirateBay) getProxy() string {
	// TODO
	return "https://thepiratebay.org"
}
