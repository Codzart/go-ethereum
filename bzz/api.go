package bzz

import (
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/resolver"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discover"
)

/*
Api implements webserver/file system related content storage and retrieval
on top of the dpa
*/
type Api struct {
	dpa      *DPA
	netStore *netStore
	Resolver *resolver.Resolver
}

func NewApi(datadir, port string) (api *Api, err error) {

	api = &Api{}

	api.netStore, err = newNetStore(filepath.Join(datadir, "bzz"), filepath.Join(datadir, "bzzpeers.json"))
	if err != nil {
		return
	}

	chunker := &TreeChunker{}
	chunker.Init()
	api.dpa = &DPA{
		Chunker:    chunker,
		ChunkStore: api.netStore,
	}
	return
}

func (self *Api) Bzz() (p2p.Protocol, error) {
	return BzzProtocol(self.netStore)
}

func (self *Api) Start(node *discover.Node, connectPeer func(string) error) {
	self.dpa.Start()
	self.netStore.Start(node, connectPeer)
}

func (self *Api) Stop() {
	self.dpa.Stop()
	self.netStore.Stop()
}

// Get uses iterative manifest retrieval and prefix matching
// to resolve path to contenwt using dpa retrieve
func (self *Api) Get(bzzpath string) (string, error) {
	return "", nil
}

// Put provides singleton manifest creation and optional name registration
// on top of dpa store
func (self *Api) Put(content, contentType, address, domain string) (string, error) {
	return "", nil
}

// Download replicates the manifest path structure on the local filesystem
// under localpath
func (self *Api) Download(bzzpath, localpath string) (string, error) {
	return "", nil
}

// Upload replicates a local directory as a manifest file and uploads it
// using dpa store
// TODO: localpath should point to a manifest
func (self *Api) Upload(localpath, address, domain string) (string, error) {
	return "", nil
}

type errResolve error

func (self *Api) resolveHost(hostport string) (contentHash Key, errR errResolve) {
	var host, port string
	var err error
	host, port, err = net.SplitHostPort(hostport)
	if err != nil {
		errR = errResolve(fmt.Errorf("invalid host '%s': %v", hostport, err))
		return
	}
	if hashMatcher.MatchString(host) {
		contentHash = Key(host)
	} else {
		if self.Resolver != nil {
			hostHash := common.BytesToHash(crypto.Sha3([]byte(host)))
			// TODO: should take port as block number versioning
			_ = port
			var hash common.Hash
			hash, err = self.Resolver.KeyToContentHash(hostHash)
			if err != nil {
				err = errResolve(fmt.Errorf("unable to resolve '%s': %v", hostport, err))
			}
			contentHash = Key(hash.Bytes())
		} else {
			err = errResolve(fmt.Errorf("no resolver '%s': %v", hostport, err))
		}
	}
	return
}

func (self *Api) getPath(uri string) (reader SectionReader, mimeType string, status int, err error) {
	parts := strings.SplitAfterN(uri[1:], "/", 2)
	hostPort := parts[0]
	path := parts[1]
	dpaLogger.Debugf("Swarm: host: '%s', path '%s' requested.", hostPort, path)

	//resolving host and port
	var key Key
	key, err = self.resolveHost(hostPort)
	if err != nil {
		return
	}

	// retrieve content following path along manifests
	var pos int
	for {
		// retrieve manifest via DPA
		manifestReader := self.dpa.Retrieve(key)
		// TODO check size for oversized manifests
		manifestData := make([]byte, manifestReader.Size())
		var size int
		size, err = manifestReader.Read(manifestData)
		if int64(size) < manifestReader.Size() {
			dpaLogger.Debugf("Swarm: Manifest for '%s' not found.", uri)
			if err == nil {
				err = fmt.Errorf("Manifest retrieval cut short: %v &lt; %v", size, manifestReader.Size())
			}
			return
		}

		dpaLogger.Debugf("Swarm: Manifest for '%s' retrieved.", uri)
		man := manifest{}
		err = json.Unmarshal(manifestData, &man)
		if err != nil {
			err = fmt.Errorf("Manifest for '%s' is malformed: %v", uri, err)
			dpaLogger.Debugf("Swarm: %v", err)
			return
		}

		dpaLogger.Debugf("Swarm: Manifest for '%s' has %d entries.", uri, len(man.Entries))

		// retrieve entry that matches path from manifest entries
		var entry *manifestEntry
		entry, pos = man.getEntry(path)
		if entry == nil {
			err = fmt.Errorf("Content for '%s' not found.", uri)
			return
		}

		// check hash of entry
		if !hashMatcher.MatchString(entry.Hash) {
			err = fmt.Errorf("Incorrect hash '%064x' for '%s'", entry.Hash, uri)
			return
		}
		key = common.Hex2Bytes(entry.Hash)
		status = entry.Status

		// get mime type of entry
		mimeType = entry.ContentType
		if mimeType != "" {
			mimeType = manifestType
		}

		// if path matched on non-manifest content type, then retrieve reader
		// and return
		if mimeType != manifestType {
			reader = self.dpa.Retrieve(key)
			return
		}

		// otherwise continue along the path with manifest resolution
		path = path[pos:]
	}
	return
}
