package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ssz "github.com/prysmaticlabs/fastssz"
	"github.com/prysmaticlabs/prysm/v3/api/client/beacon"
	"github.com/prysmaticlabs/prysm/v3/beacon-chain/rpc/apimiddleware"
	"github.com/prysmaticlabs/prysm/v3/consensus-types/primitives"
	"github.com/prysmaticlabs/prysm/v3/encoding/bytesutil"
	v1 "github.com/prysmaticlabs/prysm/v3/proto/engine/v1"
	eth "github.com/prysmaticlabs/prysm/v3/proto/prysm/v1alpha1"
	"github.com/urfave/cli/v2"
	"math/big"
	"strconv"
)

const HOST = "https://lodestar-mainnet.chainsafe.io/"

type bellatrixBlockResponseJson struct {
	Version             string                                                 `json:"version"`
	Data                *apimiddleware.SignedBeaconBlockBellatrixContainerJson `json:"data"`
	ExecutionOptimistic bool                                                   `json:"execution_optimistic"`
}

func generateCMD() *cli.Command {
	return &cli.Command{
		Name:   "generate",
		Usage:  "Generate exe proof",
		Action: generate,
		Flags: []cli.Flag{
			&cli.Uint64Flag{
				Name:     "slot",
				Usage:    "beacon chain slot",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "endpoint",
				Usage: "beacon chain endpoint",
				Value: HOST,
			},
		},
	}
}

func generate(cliCtx *cli.Context) error {
	slot := cliCtx.Uint64("slot")
	url := cliCtx.String("endpoint")
	client, err := beacon.NewClient(url)
	if err != nil {
		return err
	}

	data, err := client.GetBlock(context.Background(), beacon.StateOrBlockId(strconv.Itoa(int(slot))))
	if err != nil {
		panic(err)
	}

	blockResp := bellatrixBlockResponseJson{}
	if err := json.Unmarshal(data, &blockResp); err != nil {
		panic(err)
	}

	body := blockResp.Data.Message.Body
	depositCount, err := strconv.Atoi(body.Eth1Data.DepositCount)
	if err != nil {
		fmt.Print(err)
	}

	beaconBlockBody := eth.BeaconBlockBodyBellatrix{
		RandaoReveal: common.FromHex(body.RandaoReveal),
		Eth1Data: &eth.Eth1Data{
			DepositRoot:  common.FromHex(body.Eth1Data.DepositRoot),
			DepositCount: uint64(depositCount),
			BlockHash:    common.FromHex(body.Eth1Data.BlockHash),
		},
		Graffiti:          common.FromHex(body.Graffiti),
		ProposerSlashings: nil,
		AttesterSlashings: nil,
		Attestations:      nil,
		Deposits:          nil,
		VoluntaryExits:    nil,
		SyncAggregate: &eth.SyncAggregate{
			SyncCommitteeBits:      common.FromHex(body.SyncAggregate.SyncCommitteeBits),
			SyncCommitteeSignature: common.FromHex(body.SyncAggregate.SyncCommitteeSignature),
		},
		ExecutionPayload: &v1.ExecutionPayload{
			ParentHash:    common.FromHex(body.ExecutionPayload.ParentHash),
			FeeRecipient:  common.FromHex(body.ExecutionPayload.FeeRecipient),
			StateRoot:     common.FromHex(body.ExecutionPayload.StateRoot),
			ReceiptsRoot:  common.FromHex(body.ExecutionPayload.ReceiptsRoot),
			LogsBloom:     common.FromHex(body.ExecutionPayload.LogsBloom),
			PrevRandao:    common.FromHex(body.ExecutionPayload.PrevRandao),
			BlockNumber:   stringToUint64(body.ExecutionPayload.BlockNumber),
			GasLimit:      stringToUint64(body.ExecutionPayload.GasLimit),
			GasUsed:       stringToUint64(body.ExecutionPayload.GasUsed),
			Timestamp:     stringToUint64(body.ExecutionPayload.TimeStamp),
			ExtraData:     common.FromHex(body.ExecutionPayload.ExtraData),
			BaseFeePerGas: nil,
			BlockHash:     common.FromHex(body.ExecutionPayload.BlockHash),
			Transactions:  nil,
		},
	}

	baseFee, ret := new(big.Int).SetString(body.ExecutionPayload.BaseFeePerGas, 10)
	if !ret {
		panic("DecodeBig")
	}
	beaconBlockBody.ExecutionPayload.BaseFeePerGas = bytesutil.PadTo(bytesutil.ReverseByteOrder(baseFee.Bytes()), 32)

	for _, pSlash := range body.ProposerSlashings {
		header1 := &eth.SignedBeaconBlockHeader{
			Header:    convertToBeaconBlockHeader(pSlash.Header_1.Header),
			Signature: common.FromHex(pSlash.Header_1.Signature),
		}
		header2 := &eth.SignedBeaconBlockHeader{
			Header:    convertToBeaconBlockHeader(pSlash.Header_2.Header),
			Signature: common.FromHex(pSlash.Header_2.Signature),
		}
		beaconBlockBody.ProposerSlashings = append(beaconBlockBody.ProposerSlashings, &eth.ProposerSlashing{
			Header_1: header1,
			Header_2: header2,
		})
	}
	for _, aSlash := range body.AttesterSlashings {
		att1 := &eth.IndexedAttestation{
			AttestingIndices: stringsToUint64s(aSlash.Attestation_1.AttestingIndices),
			Data:             convertToAttestationData(aSlash.Attestation_1.Data),
			Signature:        common.FromHex(aSlash.Attestation_1.Signature),
		}
		att2 := &eth.IndexedAttestation{
			AttestingIndices: stringsToUint64s(aSlash.Attestation_2.AttestingIndices),
			Data:             convertToAttestationData(aSlash.Attestation_2.Data),
			Signature:        common.FromHex(aSlash.Attestation_2.Signature),
		}
		beaconBlockBody.AttesterSlashings = append(beaconBlockBody.AttesterSlashings, &eth.AttesterSlashing{
			Attestation_1: att1,
			Attestation_2: att2,
		})
	}

	for _, att := range body.Attestations {
		attestaton := &eth.Attestation{
			AggregationBits: common.FromHex(att.AggregationBits),
			Data: &eth.AttestationData{
				Slot:            types.Slot(stringToUint64(att.Data.Slot)),
				CommitteeIndex:  types.CommitteeIndex(stringToUint64(att.Data.CommitteeIndex)),
				BeaconBlockRoot: common.FromHex(att.Data.BeaconBlockRoot),
				Source: &eth.Checkpoint{
					Epoch: types.Epoch(stringToUint64(att.Data.Source.Epoch)),
					Root:  common.FromHex(att.Data.Source.Root),
				},
				Target: &eth.Checkpoint{
					Epoch: types.Epoch(stringToUint64(att.Data.Target.Epoch)),
					Root:  common.FromHex(att.Data.Target.Root),
				},
			},
			Signature: common.FromHex(att.Signature),
		}

		beaconBlockBody.Attestations = append(beaconBlockBody.Attestations, attestaton)
	}

	for _, dep := range body.Deposits {
		deposit := &eth.Deposit{
			Data: &eth.Deposit_Data{
				PublicKey:             common.FromHex(dep.Data.PublicKey),
				WithdrawalCredentials: common.FromHex(dep.Data.WithdrawalCredentials),
				Amount:                stringToUint64(dep.Data.Amount),
				Signature:             common.FromHex(dep.Data.Signature),
			},
		}

		for _, proof := range dep.Proof {
			deposit.Proof = append(deposit.Proof, common.FromHex(proof))
		}

		beaconBlockBody.Deposits = append(beaconBlockBody.Deposits, deposit)
	}

	for _, ve := range body.VoluntaryExits {
		voluntaryExist := &eth.SignedVoluntaryExit{
			Exit: &eth.VoluntaryExit{
				Epoch:          types.Epoch(stringToUint64(ve.Exit.Epoch)),
				ValidatorIndex: types.ValidatorIndex(stringToUint64(ve.Exit.ValidatorIndex)),
			},
			Signature: common.FromHex(ve.Signature),
		}
		beaconBlockBody.VoluntaryExits = append(beaconBlockBody.VoluntaryExits, voluntaryExist)
	}

	for _, tx := range body.ExecutionPayload.Transactions {
		beaconBlockBody.ExecutionPayload.Transactions = append(beaconBlockBody.ExecutionPayload.Transactions, common.FromHex(tx))
	}

	tree1, err := newBeaconBlockBodyTree(&beaconBlockBody)
	if err != nil {
		panic(err)
	}

	proof1, err := tree1.getExecutionPayloadProof()
	if err != nil {
		panic(err)
	}

	tree2, err := newExecutionPayloadTree(beaconBlockBody.ExecutionPayload)
	if err != nil {
		panic(err)
	}

	proof2, err := tree2.getBlockHashProof()
	if err != nil {
		panic(err)
	}

	fmt.Println("proof1 hash size", len(proof1.Hashes))

	hashes := append(proof1.Hashes, proof2.Hashes...)
	for _, hash := range hashes {
		fmt.Println(hexutil.Encode(hash[:]))
	}

	root, _ := beaconBlockBody.ExecutionPayload.HashTreeRoot()
	ret, err = ssz.VerifyProof(root[:], proof2)
	if err != nil {
		panic(err)
	}

	if !ret {
		return fmt.Errorf("VerifyProof fail")
	}

	return nil

}

func convertToBeaconBlockHeader(header *apimiddleware.BeaconBlockHeaderJson) *eth.BeaconBlockHeader {
	return &eth.BeaconBlockHeader{
		Slot:          types.Slot(stringToUint64(header.Slot)),
		ProposerIndex: types.ValidatorIndex(stringToUint64(header.ProposerIndex)),
		ParentRoot:    common.FromHex(header.ParentRoot),
		StateRoot:     common.FromHex(header.StateRoot),
		BodyRoot:      common.FromHex(header.BodyRoot),
	}
}

func convertToAttestationData(data *apimiddleware.AttestationDataJson) *eth.AttestationData {
	return &eth.AttestationData{
		Slot:            types.Slot(stringToUint64(data.Slot)),
		CommitteeIndex:  types.CommitteeIndex(stringToUint64(data.CommitteeIndex)),
		BeaconBlockRoot: common.FromHex(data.BeaconBlockRoot),
		Source: &eth.Checkpoint{
			Epoch: types.Epoch(stringToUint64(data.Source.Epoch)),
			Root:  common.FromHex(data.Source.Root),
		},
		Target: &eth.Checkpoint{
			Epoch: types.Epoch(stringToUint64(data.Target.Epoch)),
			Root:  common.FromHex(data.Target.Root),
		},
	}
}

func stringToUint64(s string) uint64 {
	i, e := strconv.ParseUint(s, 10, 64)
	if e != nil {
		panic(e)
	}

	return uint64(i)
}

func stringsToUint64s(ss []string) []uint64 {
	var ret []uint64
	for _, s := range ss {
		i, e := strconv.ParseUint(s, 10, 64)
		if e != nil {
			panic(e)
		}
		ret = append(ret, uint64(i))
	}

	return ret
}
