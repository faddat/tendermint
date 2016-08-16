package consensus

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-events"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
)

/*
	The easiest way to generate this data is to rm ~/.tendermint,
	copy ~/.tendermint_test/somedir/* to ~/.tendermint,
	run `tendermint unsafe_reset_all`,
	set ~/.tendermint/config.toml to use "leveldb" (to create a cswal in data/),
	run `make install`,
	and to run a local node.

	If you need to change the signatures, you can use a script as follows:
	The privBytes comes from config/tendermint_test/...

	```
	package main

	import (
		"encoding/hex"
		"fmt"

		"github.com/tendermint/go-crypto"
	)

	func main() {
		signBytes, err := hex.DecodeString("7B22636861696E5F6964223A2274656E6465726D696E745F74657374222C22766F7465223A7B22626C6F636B5F68617368223A2242453544373939433846353044354645383533364334333932464443384537423342313830373638222C22626C6F636B5F70617274735F686561646572223A506172745365747B543A31204236323237323535464632307D2C22686569676874223A312C22726F756E64223A302C2274797065223A327D7D")
		if err != nil {
			panic(err)
		}
		privBytes, err := hex.DecodeString("27F82582AEFAE7AB151CFB01C48BB6C1A0DA78F9BDDA979A9F70A84D074EB07D3B3069C422E19688B45CBFAE7BB009FC0FA1B1EA86593519318B7214853803C8")
		if err != nil {
			panic(err)
		}
		privKey := crypto.PrivKeyEd25519{}
		copy(privKey[:], privBytes)
		signature := privKey.Sign(signBytes)
		signatureEd25519 := signature.(crypto.SignatureEd25519)
		fmt.Printf("Signature Bytes: %X\n", signatureEd25519[:])
	}
	```
*/

var testLog = `
{"time":"2016-08-20T00:59:04.481Z","msg":[3,{"duration":0,"height":1,"round":0,"step":1}]}
{"time":"2016-08-20T00:59:04.483Z","msg":[1,{"height":1,"round":0,"step":"RoundStepPropose"}]}
{"time":"2016-08-20T00:59:04.484Z","msg":[2,{"msg":[17,{"Proposal":{"height":1,"round":0,"block_parts_header":{"total":1,"hash":"9226C78A68841731B0E853461A084D329849D73F"},"pol_round":-1,"signature":"09D241E7A02B230D06D77267979D25DE54AA67A509B07DC1023A6C6523B20B1086D83CB8E56DAC67DE7117105F57029BE84E02ACC20C1F476346C4F338803E06"}}],"peer_key":""}]}
{"time":"2016-08-20T00:59:04.484Z","msg":[2,{"msg":[19,{"Height":1,"Round":0,"Part":{"index":0,"bytes":"0101010F74656E6465726D696E745F746573740101146C5E3164DE2C800000000000000114C4B01D3810579550997AC5641E759E20D99B51C10001000100","proof":{"aunts":[]}}}],"peer_key":""}]}
{"time":"2016-08-20T00:59:04.485Z","msg":[1,{"height":1,"round":0,"step":"RoundStepPrevote"}]}
{"time":"2016-08-20T00:59:04.485Z","msg":[2,{"msg":[20,{"Vote":{"validator_address":"D028C9981F7A87F3093672BF0D5B0E2A1B3ED456","validator_index":0,"height":1,"round":0,"type":1,"block_id":{"hash":"BE09462F8C590F3C32CEA0B50E4B65B04851BF41","parts":{"total":1,"hash":"9226C78A68841731B0E853461A084D329849D73F"}},"signature":"E1592D8A9C85347479DBB3C4F600196828C33C982C049A6083BC7574E4E0AB868D97E573619C23CEA470D9B1E15D89930FBD1FBCF3181CF2364B3C3155931009"}}],"peer_key":""}]}
{"time":"2016-08-20T00:59:04.487Z","msg":[1,{"height":1,"round":0,"step":"RoundStepPrecommit"}]}
{"time":"2016-08-20T00:59:04.487Z","msg":[2,{"msg":[20,{"Vote":{"validator_address":"D028C9981F7A87F3093672BF0D5B0E2A1B3ED456","validator_index":0,"height":1,"round":0,"type":2,"block_id":{"hash":"BE09462F8C590F3C32CEA0B50E4B65B04851BF41","parts":{"total":1,"hash":"9226C78A68841731B0E853461A084D329849D73F"}},"signature":"EA525B9570A98DF620755A96B3540B84267BF00011B496D0576413A921005C678FEDC3823010ECF096EDA9BC0D2044F61802341CA0396C3C9DD6FF53FEE9F30E"}}],"peer_key":""}]}
`

// map lines in the above wal to privVal step
var mapPrivValStep = map[int]int8{
	0: 0,
	1: 0,
	2: 1,
	3: 1,
	4: 1,
	5: 2,
	6: 2,
	7: 3,
}

func writeWAL(log string) string {
	fmt.Println("writing", log)
	// write the needed wal to file
	f, err := ioutil.TempFile(os.TempDir(), "replay_test_")
	if err != nil {
		panic(err)
	}

	_, err = f.WriteString(log)
	if err != nil {
		panic(err)
	}
	name := f.Name()
	f.Close()
	return name
}

func waitForBlock(newBlockCh chan interface{}) {
	after := time.After(time.Second * 10)
	select {
	case <-newBlockCh:
	case <-after:
		panic("Timed out waiting for new block")
	}
}

func runReplayTest(t *testing.T, cs *ConsensusState, fileName string, newBlockCh chan interface{}) {
	cs.config.Set("cswal", fileName)
	cs.Start()
	// Wait to make a new block.
	// This is just a signal that we haven't halted; its not something contained in the WAL itself.
	// Assuming the consensus state is running, replay of any WAL, including the empty one,
	// should eventually be followed by a new block, or else something is wrong
	waitForBlock(newBlockCh)
	cs.Stop()
}

func setupReplayTest(nLines int, crashAfter bool) (*ConsensusState, chan interface{}, string, string) {
	fmt.Println("-------------------------------------")
	log.Notice(Fmt("Starting replay test of %d lines of WAL (crash before write)", nLines))

	lineStep := nLines
	if crashAfter {
		lineStep -= 1
	}

	split := strings.Split(testLog, "\n")
	lastMsg := split[nLines]

	// we write those lines up to (not including) one with the signature
	fileName := writeWAL(strings.Join(split[:nLines], "\n") + "\n")

	cs := fixedConsensusState()

	// set the last step according to when we crashed vs the wal
	cs.privValidator.LastHeight = 1 // first block
	cs.privValidator.LastStep = mapPrivValStep[lineStep]

	fmt.Println("LAST STEP", cs.privValidator.LastStep)

	newBlockCh := subscribeToEvent(cs.evsw, "tester", types.EventStringNewBlock(), 1)

	return cs, newBlockCh, lastMsg, fileName
}

//-----------------------------------------------
// Test the log at every iteration, and set the privVal last step
// as if the log was written after signing, before the crash

func TestReplayCrashAfterWrite(t *testing.T) {
	split := strings.Split(testLog, "\n")
	for i := 0; i < len(split)-1; i++ {
		cs, newBlockCh, _, f := setupReplayTest(i+1, true)
		runReplayTest(t, cs, f, newBlockCh)
	}
}

//-----------------------------------------------
// Test the log as if we crashed after signing but before writing.
// This relies on privValidator.LastSignature being set

func TestReplayCrashBeforeWritePropose(t *testing.T) {
	cs, newBlockCh, proposalMsg, f := setupReplayTest(2, false) // propose
	// Set LastSig
	var err error
	var msg ConsensusLogMessage
	wire.ReadJSON(&msg, []byte(proposalMsg), &err)
	proposal := msg.Msg.(msgInfo).Msg.(*ProposalMessage)
	if err != nil {
		t.Fatalf("Error reading json data: %v", err)
	}
	cs.privValidator.LastSignBytes = types.SignBytes(cs.state.ChainID, proposal.Proposal)
	cs.privValidator.LastSignature = proposal.Proposal.Signature
	runReplayTest(t, cs, f, newBlockCh)
}

func TestReplayCrashBeforeWritePrevote(t *testing.T) {
	cs, newBlockCh, voteMsg, f := setupReplayTest(5, false) // prevote
	cs.evsw.AddListenerForEvent("tester", types.EventStringCompleteProposal(), func(data events.EventData) {
		// Set LastSig
		var err error
		var msg ConsensusLogMessage
		wire.ReadJSON(&msg, []byte(voteMsg), &err)
		vote := msg.Msg.(msgInfo).Msg.(*VoteMessage)
		if err != nil {
			t.Fatalf("Error reading json data: %v", err)
		}
		cs.privValidator.LastSignBytes = types.SignBytes(cs.state.ChainID, vote.Vote)
		cs.privValidator.LastSignature = vote.Vote.Signature
	})
	runReplayTest(t, cs, f, newBlockCh)
}

func TestReplayCrashBeforeWritePrecommit(t *testing.T) {
	cs, newBlockCh, voteMsg, f := setupReplayTest(7, false) // precommit
	cs.evsw.AddListenerForEvent("tester", types.EventStringPolka(), func(data events.EventData) {
		// Set LastSig
		var err error
		var msg ConsensusLogMessage
		wire.ReadJSON(&msg, []byte(voteMsg), &err)
		vote := msg.Msg.(msgInfo).Msg.(*VoteMessage)
		if err != nil {
			t.Fatalf("Error reading json data: %v", err)
		}
		cs.privValidator.LastSignBytes = types.SignBytes(cs.state.ChainID, vote.Vote)
		cs.privValidator.LastSignature = vote.Vote.Signature
	})
	runReplayTest(t, cs, f, newBlockCh)
}
