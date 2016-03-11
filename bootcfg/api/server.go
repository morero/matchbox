package api

import (
	"net/http"

	"github.com/coreos/coreos-baremetal/bootcfg/sign"
	"github.com/coreos/coreos-baremetal/bootcfg/storage"
	"github.com/coreos/pkg/capnslog"
)

const (
	// APIVersion of the api server and its config types.
	APIVersion = "v1alpha1"
)

var log = capnslog.NewPackageLogger("github.com/coreos/coreos-baremetal/bootcfg", "api")

// Config configures the api Server.
type Config struct {
	// Store for configs
	Store storage.Store
	// Path to static assets
	AssetsPath string
	// config signers (.sig and .asc)
	Signer        sign.Signer
	ArmoredSigner sign.Signer
}

// Server serves boot and provisioning configs to machines.
type Server struct {
	store         storage.Store
	assetsPath    string
	signer        sign.Signer
	armoredSigner sign.Signer
}

// NewServer returns a new Server.
func NewServer(config *Config) *Server {
	return &Server{
		store:         config.Store,
		assetsPath:    config.AssetsPath,
		signer:        config.Signer,
		armoredSigner: config.ArmoredSigner,
	}
}

// HTTPHandler returns a HTTP handler for the server.
func (s *Server) HTTPHandler() http.Handler {
	mux := http.NewServeMux()
	gr := newGroupsResource(s.store)

	// Boot via GRUB
	mux.Handle("/grub", logRequests(NewHandler(gr.matchProfileHandler(grubHandler()))))
	// Boot via iPXE
	mux.Handle("/boot.ipxe", logRequests(ipxeInspect()))
	mux.Handle("/boot.ipxe.0", logRequests(ipxeInspect()))
	mux.Handle("/ipxe", logRequests(NewHandler(gr.matchProfileHandler(ipxeHandler()))))
	// Boot via Pixiecore
	mux.Handle("/pixiecore/v1/boot/", logRequests(pixiecoreHandler(gr, s.store)))
	// Ignition Config
	mux.Handle("/ignition", logRequests(NewHandler(gr.matchGroupHandler(ignitionHandler(s.store)))))
	// Cloud-Config
	mux.Handle("/cloud", logRequests(NewHandler(gr.matchGroupHandler(cloudHandler(s.store)))))
	// metadata
	mux.Handle("/metadata", logRequests(NewHandler(gr.matchGroupHandler(metadataHandler()))))

	// Signatures
	if s.signer != nil {
		signerChain := func(next http.Handler) http.Handler {
			return logRequests(sign.SignatureHandler(s.signer, next))
		}
		mux.Handle("/grub.sig", signerChain(NewHandler(gr.matchProfileHandler(grubHandler()))))
		mux.Handle("/boot.ipxe.sig", signerChain(ipxeInspect()))
		mux.Handle("/boot.ipxe.0.sig", signerChain(ipxeInspect()))
		mux.Handle("/ipxe.sig", signerChain(NewHandler(gr.matchProfileHandler(ipxeHandler()))))
		mux.Handle("/pixiecore/v1/boot.sig/", signerChain(pixiecoreHandler(gr, s.store)))
		mux.Handle("/ignition.sig", signerChain(NewHandler(gr.matchGroupHandler(ignitionHandler(s.store)))))
		mux.Handle("/cloud.sig", signerChain(NewHandler(gr.matchGroupHandler(cloudHandler(s.store)))))
		mux.Handle("/metadata.sig", signerChain(NewHandler(gr.matchGroupHandler(metadataHandler()))))
	}
	if s.armoredSigner != nil {
		signerChain := func(next http.Handler) http.Handler {
			return logRequests(sign.SignatureHandler(s.armoredSigner, next))
		}
		mux.Handle("/grub.asc", signerChain(NewHandler(gr.matchProfileHandler(grubHandler()))))
		mux.Handle("/boot.ipxe.asc", signerChain(ipxeInspect()))
		mux.Handle("/boot.ipxe.0.asc", signerChain(ipxeInspect()))
		mux.Handle("/ipxe.asc", signerChain(NewHandler(gr.matchProfileHandler(ipxeHandler()))))
		mux.Handle("/pixiecore/v1/boot.asc/", signerChain(pixiecoreHandler(gr, s.store)))
		mux.Handle("/ignition.asc", signerChain(NewHandler(gr.matchGroupHandler(ignitionHandler(s.store)))))
		mux.Handle("/cloud.asc", signerChain(NewHandler(gr.matchGroupHandler(cloudHandler(s.store)))))
		mux.Handle("/metadata.asc", signerChain(NewHandler(gr.matchGroupHandler(metadataHandler()))))
	}

	// kernel, initrd, and TLS assets
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(s.assetsPath))))
	return mux
}
