/*
	Copyright (C) CESS. All rights reserved.
	Copyright (C) Cumulus Encrypted Storage System. All rights reserved.

	SPDX-License-Identifier: Apache-2.0
*/

package node

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/CESSProject/DeOSS/configs"
	"github.com/CESSProject/DeOSS/pkg/utils"
	"github.com/CESSProject/cess-go-sdk/chain"
	sconfig "github.com/CESSProject/cess-go-sdk/config"
	"github.com/CESSProject/cess-go-sdk/core/process"
	sutils "github.com/CESSProject/cess-go-sdk/utils"
	"github.com/mr-tron/base58"
	"github.com/pkg/errors"
)

type RecordInfo struct {
	SegmentInfo []chain.SegmentDataInfo `json:"segmentInfo"`
	Owner       []byte                  `json:"owner"`
	Roothash    string                  `json:"roothash"`
	Filename    string                  `json:"filename"`
	Buckname    string                  `json:"buckname"`
	Filesize    uint64                  `json:"filesize"`
	Putflag     bool                    `json:"putflag"`
	Count       uint8                   `json:"count"`
	Duplicate   bool                    `json:"duplicate"`
}

// MinRecordInfoLength = len(json.Marshal(RecordInfo{}))
const MinRecordInfoLength = 132

// tracker
func (n *Node) tracker(ch chan<- bool) {
	defer func() {
		ch <- true
		if err := recover(); err != nil {
			n.Pnc(utils.RecoverError(err))
		}
	}()

	n.Track("info", ">>>>> start tracker <<<<<")

	var err error
	var processNum int
	var trackFiles []string

	for {
		trackFiles, err = n.ListTrackFiles()
		if err != nil {
			n.Track("err", err.Error())
			time.Sleep(chain.BlockInterval)
			continue
		}
		if len(trackFiles) == 0 {
			time.Sleep(time.Minute)
			continue
		}
		processNum = n.GetTrackFileNum()

		if processNum < configs.MaxTrackThread {
			for _, v := range trackFiles {
				if _, err = os.Stat(v); err != nil {
					continue
				}
				err = n.AddTrackFile(filepath.Base(v))
				if err != nil {
					continue
				}
				n.Track("info", fmt.Sprintf("start track file: %s", filepath.Base(v)))
				go func(file string) { n.trackFileThread(file) }(v)
				time.Sleep(chain.BlockInterval)
			}
		}
		time.Sleep(time.Minute)
	}
}

func (n *Node) trackFileThread(trackFile string) {
	defer func() {
		n.DelTrackFile(filepath.Base(trackFile))
	}()
	err := n.trackFile(trackFile)
	if err != nil {
		n.Track("err", err.Error())
	}
	n.Track("info", fmt.Sprintf("end track file: %s", filepath.Base(trackFile)))
}

func (n *Node) trackFile(trackfile string) error {
	var (
		err          error
		roothash     string
		recordFile   RecordInfo
		storageOrder chain.StorageOrder
	)
	roothash = filepath.Base(trackfile)
	for {
		recordFile, err = n.ParseTrackFile(roothash)
		if err != nil {
			return errors.Wrapf(err, "[ParseTrackFromFile]")
		}

		_, err = n.QueryFile(roothash, -1)
		if err != nil {
			if err.Error() != chain.ERR_Empty {
				time.Sleep(time.Second * chain.BlockInterval)
				return errors.Wrapf(err, "[%s] [QueryFileMetadata]", roothash)
			}
		} else {
			n.Track("info", fmt.Sprintf("[%s] storage successful", roothash))
			if len(recordFile.SegmentInfo) > 0 {
				baseDir := filepath.Dir(recordFile.SegmentInfo[0].SegmentHash)
				//os.Rename(filepath.Join(baseDir, roothash), filepath.Join(n.GetDirs().FileDir, roothash))
				os.RemoveAll(filepath.Join(n.GetDirs().FileDir, roothash))
				os.RemoveAll(baseDir)
			}
			n.DeleteTrackFile(roothash) // if storage successfully ,remove track file
			return nil
		}

		storageOrder, err = n.QueryDealMap(roothash, -1)
		if err != nil {
			if err.Error() != chain.ERR_Empty {
				return errors.Wrapf(err, "[%s] [QueryStorageOrder]", roothash)
			}
			recordFile.Putflag = false
			recordFile.Count = 0
			b, err := json.Marshal(&recordFile)
			if err != nil {
				return errors.Wrapf(err, "[%s] [json.Marshal]", roothash)
			}
			err = n.WriteTrackFile(roothash, b)
			if err != nil {
				return errors.Wrapf(err, "[%s] [WriteTrackFile]", roothash)
			}

			// verify the space is authorized
			authAccs, err := n.QueryAuthorityList(recordFile.Owner, -1)
			if err != nil {
				if err.Error() != chain.ERR_Empty {
					return errors.Wrapf(err, "[%s] [QuaryAuthorizedAccount]", roothash)
				}
			}
			var flag bool
			for _, v := range authAccs {
				if sutils.CompareSlice(n.GetSignatureAccPulickey(), v[:]) {
					flag = true
					break
				}
			}
			if !flag {
				if len(recordFile.SegmentInfo) > 0 {
					baseDir := filepath.Dir(recordFile.SegmentInfo[0].SegmentHash)
					os.RemoveAll(baseDir)
				}
				n.DeleteTrackFile(roothash)
				user, _ := sutils.EncodePublicKeyAsCessAccount(recordFile.Owner)
				return errors.Errorf("[%s] user [%s] deauthorization", roothash, user)
			}

			txhash, err := n.GenerateStorageOrder(
				roothash,
				recordFile.SegmentInfo,
				recordFile.Owner,
				recordFile.Filename,
				recordFile.Buckname,
				recordFile.Filesize,
			)
			if err != nil {
				return errors.Wrapf(err, "[%s] [%s] [GenerateStorageOrder]", txhash, roothash)
			}
			n.Track("info", fmt.Sprintf("[%s] GenerateStorageOrder: %s", roothash, txhash))
			time.Sleep(chain.BlockInterval * 3)
			storageOrder, err = n.QueryDealMap(roothash, -1)
			if err != nil {
				return errors.Wrapf(err, "[%s] [QueryStorageOrder]", roothash)
			}
		}

		if roothash != recordFile.Roothash {
			n.DeleteTrackFile(roothash)
			return errors.Errorf("[%s] Recorded filehash [%s] error", roothash, recordFile.Roothash)
		}

		// if recordFile.Putflag {
		// 	if storageorder.AssignedMiner != nil {
		// 		if uint8(storageorder.Count) == recordFile.Count {
		// 			return nil
		// 		}
		// 	}
		// }

		if recordFile.Duplicate {
			_, err = n.QueryFile(roothash, -1)
			if err == nil {
				_, err = n.GenerateStorageOrder(recordFile.Roothash, nil, recordFile.Owner, recordFile.Filename, recordFile.Buckname, recordFile.Filesize)
				if err != nil {
					return errors.Wrapf(err, " [%s] [GenerateStorageOrder]", roothash)
				}
				if len(recordFile.SegmentInfo) > 0 {
					baseDir := filepath.Dir(recordFile.SegmentInfo[0].SegmentHash)
					os.Rename(filepath.Join(baseDir, roothash), filepath.Join(n.GetDirs().FileDir, roothash))
					os.RemoveAll(baseDir)
				}
				n.DeleteTrackFile(roothash)
				n.Track("info", fmt.Sprintf("[%s] Duplicate file declaration suc", roothash))
				return nil
			}
			storageOrder, err = n.QueryDealMap(recordFile.Roothash, -1)
			if err != nil {
				if err.Error() != chain.ERR_Empty {
					return errors.Wrapf(err, "[%s] [QueryStorageOrder]", roothash)
				}
				n.Track("info", fmt.Sprintf("[%s] Duplicate file become primary file", roothash))
				recordFile.Duplicate = false
				recordFile.Putflag = false
				b, err := json.Marshal(&recordFile)
				if err != nil {
					return errors.Wrapf(err, "[%s] [json.Marshal]", roothash)
				}
				err = n.WriteTrackFile(roothash, b)
				if err != nil {
					return errors.Wrapf(err, "[%s] [WriteTrackFile]", roothash)
				}
			}
			return nil
		}

		if recordFile.SegmentInfo == nil {
			resegmentInfo, reHash, err := process.ShardedEncryptionProcessing(filepath.Join(n.GetDirs().FileDir, roothash), "")
			if err != nil {
				return errors.Wrapf(err, "[ShardedEncryptionProcessing]")
			}
			if reHash != roothash {
				return errors.Wrapf(err, "The re-stored file hash is not consistent, please store it separately and specify the original encryption key.")
			}
			recordFile.SegmentInfo = resegmentInfo
		}

		err = n.storageData(recordFile.Roothash, recordFile.SegmentInfo, storageOrder.CompleteList)
		n.FlushlistedPeerNodes(5*time.Second, n.GetDHTable()) //refresh the user-configured storage node list
		if err != nil {
			n.Track("err", err.Error())
			return err
		} else {
			n.Track("info", fmt.Sprintf("[%s] file transfer completed", roothash))
			time.Sleep(time.Minute * 3)
		}
	}
}

func (n *Node) storageData(roothash string, segment []chain.SegmentDataInfo, completeList []chain.CompleteInfo) error {
	var err error
	var failed bool
	var completed bool
	var dataGroup = make(map[uint8][]string, (sconfig.DataShards + sconfig.ParShards))
	for index := 0; index < len(segment[0].FragmentHash); index++ {
		for i := 0; i < len(segment); i++ {
			dataGroup[uint8(index+1)] = append(dataGroup[uint8(index+1)], string(segment[i].FragmentHash[index]))
		}
	}

	//allpeers := n.GetAllStoragePeerId()
	itor, err := n.NewPeersIterator(sconfig.DataShards + sconfig.ParShards)
	if err != nil {
		return err
	}

	//n.Track("info", fmt.Sprintf("All storage peers: %v", allpeers))
	var sucPeer = make(map[string]struct{}, sconfig.DataShards+sconfig.ParShards)

	for _, value := range completeList {
		minfo, err := n.QueryMinerItems(value.Miner[:], -1)
		if err != nil {
			continue
		}
		sucPeer[base58.Encode([]byte(string(minfo.PeerId[:])))] = struct{}{}
	}

	for index, v := range dataGroup {
		completed = false
		for _, value := range completeList {
			if uint8(value.Index) == index {
				completed = true
				n.Track("info", fmt.Sprintf("[%s] The %dth batch fragments already report", roothash, index))
				break
			}
		}

		if completed {
			continue
		}

		n.Track("info", fmt.Sprintf("[%s] Prepare to transfer the %dth batch of fragments (%d)", roothash, index, len(v)))
		//utils.RandSlice(allpeers)
		for peer, ok := itor.GetPeer(); ok; peer, ok = itor.GetPeer() {
			failed = true
			if _, ok := sucPeer[peer.ID.String()]; ok {
				continue
			}

			err = n.Connect(context.TODO(), peer)
			if err != nil {
				n.Feedback(peer.ID.String(), false)
				continue
			}

			n.Track("info", fmt.Sprintf("[%s] Will transfer to %s", roothash, peer.ID.String()))
			for j := 0; j < len(v); j++ {
				for k := 0; k < 10; k++ {
					err = n.WriteFileAction(peer.ID, roothash, v[j])
					if err != nil {
						time.Sleep(chain.BlockInterval * 3)
						continue
					}
					failed = false
					break
				}
				if failed {
					n.Track("err", fmt.Sprintf("[%s] transfer to %s failed: %v", roothash, peer.ID.String(), err))
					n.Feedback(peer.ID.String(), false)
					break
				}
				n.Track("info", fmt.Sprintf("[%s] The %dth fragment of the %dth batch is transferred to %s", roothash, j, index, peer.ID.String()))
			}
			if !failed {
				sucPeer[peer.ID.String()] = struct{}{}
				n.Feedback(peer.ID.String(), true)
				n.Track("info", fmt.Sprintf("[%s] The %dth batch of fragments is transferred to %s", roothash, index, peer.ID.String()))
				break
			}
		}
	}

	return nil
}
