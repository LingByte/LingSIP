package main

// Demonstrates sdp.Parse + sdp.BuildAudioAnswer (offer → answer helper).
//
//	cd lingsip && go run ./examples/sdp-answer

import (
	"fmt"

	"github.com/LingByte/lingsip/sdp"
)

func main() {
	offer := sdp.Generate("203.0.113.10", 4000, []sdp.Codec{
		{PayloadType: 111, Name: sdp.CodecOpus, ClockRate: 48000},
		{PayloadType: 0, Name: sdp.CodecPCMU, ClockRate: 8000},
		{PayloadType: 8, Name: sdp.CodecPCMA, ClockRate: 8000},
		{PayloadType: 101, Name: sdp.CodecTelephoneEvent, ClockRate: 8000},
	})
	fmt.Println("=== offer ===")
	fmt.Print(offer)

	info, err := sdp.Parse(offer)
	if err != nil {
		panic(err)
	}
	fmt.Printf("parsed offer: %s:%d proto=%s codecs=", info.IP, info.Port, info.Proto)
	for i, c := range info.Codecs {
		if i > 0 {
			fmt.Print(",")
		}
		fmt.Print(c.Name)
	}
	fmt.Println()

	answer := sdp.BuildAudioAnswer("198.51.100.1", 10000, info)
	fmt.Println("=== answer (BuildAudioAnswer) ===")
	fmt.Print(answer)

	picked := sdp.SelectAnswerCodecs(info)
	fmt.Print("selected: ")
	for i, c := range picked {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Printf("%s/%d (pt=%d)", c.Name, c.ClockRate, c.PayloadType)
	}
	fmt.Println()
}
