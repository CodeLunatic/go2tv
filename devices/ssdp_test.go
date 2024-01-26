package devices

import (
	"github.com/alexballas/go-ssdp"
	"log"
	"testing"
	"time"
)

func TestAnotherSearch(t *testing.T) {
	time.Sleep(5 * time.Second)
	searches := AnotherSearch(ssdp.All, 5)
	log.Println(searches)
}

func TestSearch(t *testing.T) {
	searches, err := Search(ssdp.All, 5, "")
	if err != nil {
		return
	}
	log.Println(searches)
}
