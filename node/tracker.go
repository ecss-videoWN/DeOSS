/*
	Copyright (C) CESS. All rights reserved.
	Copyright (C) Cumulus Encrypted Storage System. All rights reserved.

	SPDX-License-Identifier: Apache-2.0
*/

package node

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/CESSProject/DeOSS/configs"
	"github.com/CESSProject/DeOSS/pkg/utils"
	"github.com/CESSProject/sdk-go/core/pattern"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mr-tron/base58"
)

func (n *Node) trackFile(ch chan<- bool) {
	defer func() {
		ch <- true
		if err := recover(); err != nil {
			n.Pnc(utils.RecoverError(err))
		}
	}()

	var (
		count         uint8
		roothash      string
		ownerAcc      string
		recordFile    RecordInfo
		storageorder  pattern.StorageOrder
		linuxFileAttr *syscall.Stat_t
	)

	for {
		time.Sleep(pattern.BlockInterval)
		files, _ := filepath.Glob(fmt.Sprintf("%s/*", n.TrackDir))
		for i := 0; i < len(files); i++ {
			roothash = filepath.Base(files[i])
			b, err := n.Cache.Get([]byte("transfer:" + roothash))
			if err == nil {
				storageorder, err = n.QueryStorageOrder(roothash)
				if err != nil {
					if err.Error() != pattern.ERR_Empty {
						n.Upfile("err", err.Error())
						continue
					}

					meta, err := n.QueryFileMetadata(roothash)
					if err != nil {
						if err.Error() != pattern.ERR_Empty {
							n.Upfile("err", err.Error())
							continue
						}
					} else {
						if meta.State == Active {
							recordFile, err = parseRecordInfoFromFile(files[i])
							if err == nil {
								ownerAcc, err = utils.EncodePublicKeyAsCessAccount(recordFile.Owner)
								if err == nil {
									for _, segment := range meta.SegmentList {
										os.Remove(filepath.Join(n.GetDirs().FileDir, ownerAcc, string(segment.Hash[:])))
										for _, fragment := range segment.FragmentList {
											os.Remove(filepath.Join(n.GetDirs().FileDir, ownerAcc, string(fragment.Hash[:])))
										}
									}
								}
							}
							os.Remove(files[i])
						}
					}
					continue
				}
			}

			recordFile, err = parseRecordInfoFromFile(files[i])
			if err != nil {
				n.Upfile("info", fmt.Sprintf("[%s] File backup failed: %v", roothash, err))
				os.Remove(files[i])
				continue
			}

			if roothash != recordFile.Roothash {
				n.Upfile("info", fmt.Sprintf("[%s] File backup failed: fid is not equal", roothash))
				os.Remove(files[i])
				continue
			}

			if recordFile.Putflag {
				if storageorder.AssignedMiner != nil {
					if uint8(storageorder.Count) == recordFile.Count {
						continue
					}
				}
			}

			count, err = n.backupFiles(recordFile.Owner, recordFile.SegmentInfo, roothash, recordFile.Filename, recordFile.Buckname)
			if err != nil {
				n.Upfile("err", fmt.Sprintf("[%v] %v", roothash, err))
				continue
			}

			n.Upfile("info", fmt.Sprintf("File [%s] backup suc", roothash))

			recordFile.Putflag = true
			recordFile.Count = count
			b, err = json.Marshal(&recordFile)
			if err != nil {
				n.Upfile("err", fmt.Sprintf("[%v] %v", roothash, err))
				continue
			}

			f, err := os.OpenFile(filepath.Join(n.TrackDir, roothash), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
			if err != nil {
				n.Upfile("err", fmt.Sprintf("[%v] %v", roothash, err))
				continue
			}
			_, err = f.Write(b)
			if err != nil {
				f.Close()
				n.Upfile("err", fmt.Sprintf("[%v] %v", roothash, err))
				continue
			}

			err = f.Sync()
			if err != nil {
				f.Close()
				n.Upfile("err", fmt.Sprintf("[%v] %v", roothash, err))
				continue
			}
			f.Close()
			n.Cache.Put([]byte("transfer:"+roothash), []byte(fmt.Sprintf("%v", count)))
		}

		// Delete files that have not been accessed for more than 30 days
		files, _ = filepath.Glob(filepath.Join(n.GetDirs().FileDir, "/*"))
		for _, v := range files {
			fs, err := os.Stat(v)
			if err == nil {
				linuxFileAttr = fs.Sys().(*syscall.Stat_t)
				if time.Since(time.Unix(linuxFileAttr.Atim.Sec, 0)).Hours() > configs.FileCacheExpirationTime {
					os.Remove(v)
				}
			}
		}
	}
}

func (n *Node) backupFiles(owner []byte, segmentInfo []pattern.SegmentDataInfo, roothash, filename, bucketname string) (uint8, error) {
	var err error
	var storageOrder pattern.StorageOrder

	_, err = n.QueryFileMetadata(roothash)
	if err == nil {
		return 0, nil
	}

	for i := 0; i < 3; i++ {
		storageOrder, err = n.QueryStorageOrder(roothash)
		if err != nil {
			if err.Error() == pattern.ERR_Empty {
				_, err = n.GenerateStorageOrder(roothash, segmentInfo, owner, filename, bucketname)
				if err != nil {
					return 0, err
				}
			}
			time.Sleep(pattern.BlockInterval)
			continue
		}
		break
	}
	if err != nil {
		return 0, err
	}

	// store fragment to storage
	err = n.storageData(roothash, segmentInfo, storageOrder.AssignedMiner)
	if err != nil {
		return 0, err
	}
	return uint8(storageOrder.Count), nil
}

func (n *Node) storageData(roothash string, segment []pattern.SegmentDataInfo, minerTaskList []pattern.MinerTaskList) error {
	var err error
	var fpath string
	// query all assigned miner multiaddr
	peerids, err := n.QueryAssignedMiner(minerTaskList)
	if err != nil {
		return err
	}

	basedir := filepath.Dir(segment[0].FragmentHash[0])
	for i := 0; i < len(peerids); i++ {
		if !n.Has(peerids[i]) {
			return fmt.Errorf("No allocated storage node found: %s", peerids[i])
		}

		id, _ := peer.Decode(peerids[i])

		for j := 0; j < len(minerTaskList[i].Hash); j++ {
			fpath = filepath.Join(basedir, string(minerTaskList[i].Hash[j][:]))
			_, err = os.Stat(fpath)
			if err != nil {
				err = utils.CopyFile(filepath.Join(basedir, roothash), filepath.Join(n.GetDirs().FileDir, roothash))
				if err != nil {
					return err
				}
				_, _, err = n.ProcessingData(filepath.Join(basedir, roothash))
				if err != nil {
					return err
				}
			}
			err = n.WriteFileAction(id, roothash, fpath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (n *Node) QueryAssignedMiner(minerTaskList []pattern.MinerTaskList) ([]string, error) {
	var peerids = make([]string, len(minerTaskList))
	for i := 0; i < len(minerTaskList); i++ {
		minerInfo, err := n.QueryStorageMiner(minerTaskList[i].Account[:])
		if err != nil {
			return peerids, err
		}
		peerids[i] = base58.Encode([]byte(string(minerInfo.PeerId[:])))
	}
	return peerids, nil
}

func parseRecordInfoFromFile(file string) (RecordInfo, error) {
	var result RecordInfo
	b, err := os.ReadFile(file)
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(b, &result)
	return result, err
}
