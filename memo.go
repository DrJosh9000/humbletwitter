package main

/*
#cgo LDFLAGS: -L${SRCDIR} -latalk
#include <stdlib.h>
#include <errno.h>
#include <netatalk/endian.h>
#include <netatalk/at.h>
#include <atalk/atp.h>
#include <atalk/netddp.h>
#include <atalk/nbp.h>
#include <atalk/util.h>
#include <atalk/unicode.h>

const charset_t kChMac = CH_MAC;
const charset_t kChUnix = CH_UNIX;
const size_t kSizeMax = SIZE_MAX;
const u_int8_t kATAddrAnyPort = ATADDR_ANYPORT;

// C can't have Go pointers to Go pointers...

struct sockaddr_at * make_sockaddr_at() {
	return (struct sockaddr_at *)malloc(sizeof(struct sockaddr_at));
}

struct iovec * make_iovec() {
	return (struct iovec *)malloc(sizeof(struct iovec));
}

// The ATP block contains a union which is painful to use in straight Go.
// Hence we need simple helpers.

void set_rreqdata(struct atp_block *atpb, void *buf, int len) {
	atpb->atp_rreqdata = buf;
	atpb->atp_rreqdlen = len;
}

int get_rreqdlen(struct atp_block *atpb) {
	return atpb->atp_rreqdlen;
}

void set_sresdata(struct atp_block *atpb, struct iovec * iov, int iovcnt) {
	atpb->atp_sresiov = iov;
	atpb->atp_sresiovcnt = iovcnt;
}
*/
import "C"

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"unsafe"

	"github.com/ChimeraCoder/anaconda"
)

var (
	api *anaconda.TwitterApi

	nbpName           = flag.String("nbp_name", "GophersInYourAppletalk:Twitter", "Name to register for NBP")
	standardResponse  = flag.String("standard_response", "Gophers in your HyperCard!", "Standard response to ATP requests")
	enableTweeting    = flag.Bool("enable_tweeting", false, "Turns on the Twitter functionality; input will be tweeted")
	twitterParamsFile = flag.String("twitter_params", "twitter_params.json", "Twitter parameters in JSON file")
	twitterTokenFile  = flag.String("twitter_token", "twitter_token.json", "Twitter user token in JSON file")
)

type oauthToken struct {
	Token  string `json:"oauth_token"`
	Secret string `json:"oauth_token_secret"`
}

func doOAuth() (*anaconda.TwitterApi, error) {
	u, c, err := anaconda.AuthorizationURL("")
	if err != nil {
		return nil, err
	}
	fmt.Printf("Go to %s in your Twitter browser session and then enter the PIN: ", u)
	var p string
	if _, err := fmt.Scanf("%s", &p); err != nil {
		return nil, err
	}
	_, v, err := anaconda.GetCredentials(c, p)
	if err != nil {
		return nil, err
	}
	t := oauthToken{
		Token:  v.Get("oauth_token"),
		Secret: v.Get("oauth_token_secret"),
	}
	if err := saveToken(*twitterTokenFile, &t); err != nil {
		return nil, err
	}
	return anaconda.NewTwitterApi(t.Token, t.Secret), nil
}

func saveToken(path string, t *oauthToken) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(t)
}

func twitterAPI() (*anaconda.TwitterApi, error) {
	var t oauthToken
	f, err := os.Open(*twitterTokenFile)
	if err != nil {
		log.Printf("Cannot open Twitter token file: %v", err)
		return doOAuth()
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&t); err != nil {
		log.Printf("Cannot decode Twitter token file: %v", err)
		f.Close()
		return doOAuth()
	}
	return anaconda.NewTwitterApi(t.Token, t.Secret), nil
}

func main() {
	flag.Parse()
	if *enableTweeting {
		log.Print("Loading Twitter consumer key & secret")
		f, err := os.Open(*twitterParamsFile)
		if err != nil {
			log.Fatalf("Cannot load Twitter parameters: %v", err)
		}
		defer f.Close()
		var p struct {
			ConsumerKey    string `json:"consumer_key"`
			ConsumerSecret string `json:"consumer_secret"`
		}
		if err := json.NewDecoder(f).Decode(&p); err != nil {
			log.Fatalf("Cannot load Twitter parameters: %v", err)
		}
		anaconda.SetConsumerKey(p.ConsumerKey)
		anaconda.SetConsumerSecret(p.ConsumerSecret)

		log.Printf("Loading user OAuth token & secret")
		a, err := twitterAPI()
		if err != nil {
			log.Fatalf("Cannot obtain OAuth token: %v", err)
		}
		api = a
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// Make sure nbpName is a useful looking name, I think?
	cn := C.CString(*nbpName)
	defer C.free(unsafe.Pointer(cn))
	var convname *C.char
	if C.convert_string_allocate(C.kChUnix, C.kChMac, unsafe.Pointer(cn), C.kSizeMax, &convname) == C.kSizeMax {
		convname = cn
	}

	var o, t, z *C.char
	if s := C.nbp_name(convname, &o, &t, &z); s != 0 {
		log.Fatalf("nbp_name: the name was wrong: %v", s)
	}

	// Open ATP on an AppleTalk (DDP) port.
	sat := C.make_sockaddr_at()
	defer C.free(unsafe.Pointer(sat))

	atAddr := (*C.struct_at_addr)(unsafe.Pointer(&sat.sat_addr))
	atp, err := C.atp_open(C.kATAddrAnyPort, atAddr)
	if atp == nil || err != nil {
		log.Fatalf("atp_open: %v", err)
	}
	defer C.atp_close(atp)

	// Register the NBP name.
	addr := &atp.atph_saddr
	n := fmt.Sprintf("%s:%s@%s", C.GoString(o), C.GoString(t), C.GoString(z))
	log.Printf("Registering NBP name %s", n)
	if r := C.nbp_rgstr(addr, o, t, z); r < 0 {
		log.Fatalf("Couldn't register %s\n: %d", n, r)
	}
	defer C.nbp_unrgstr(o, t, z, atAddr)

	// Serving loop.
	go func() {
		for {
			if err := handleOne(sat, atp); err != nil {
				log.Printf("Error handling request: %v", err)
			}
		}
	}()

	<-interrupt
	log.Println("SIGINT received, quitting...")
	C.nbp_unrgstr(o, t, z, atAddr)
}

func handleOne(sat *C.struct_sockaddr_at, atp *C.struct_atp_handle) error {
	log.Println("Awaiting next request")
	const bufSize = 4624
	buf := C.malloc(bufSize)
	defer C.free(buf)
	atpb := &C.struct_atp_block{
		atp_saddr: sat,
	}
	C.set_rreqdata(atpb, buf, bufSize)
	if s := C.atp_rreq(atp, atpb); s < 0 {
		return fmt.Errorf("atp_rreq: %v", s)
	}
	bt := C.GoBytes(buf, C.get_rreqdlen(atpb))
	log.Printf("Got request: %s", bt)

	if api != nil {
		twt := strings.TrimPrefix(string(bt), "REQS")
		if _, err := api.PostTweet(twt, nil); err != nil {
			log.Printf("Error posting tweet: %v", err)
		}
	}

	cmsg := fmt.Sprintf("RESP%s", *standardResponse)
	resp := unsafe.Pointer(C.CString(cmsg))
	defer C.free(resp)
	iov := C.make_iovec()
	defer C.free(unsafe.Pointer(iov))
	iov.iov_base = resp
	iov.iov_len = C.size_t(len(cmsg) + 1)
	C.set_sresdata(atpb, iov, 1)
	if s := C.atp_sresp(atp, atpb); s < 0 {
		return fmt.Errorf("atp_sresp: %v", s)
	}
	log.Printf("Responded with: %s", *standardResponse)
	return nil
}
