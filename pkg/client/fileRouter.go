/*
   Copyright 2022 CESS (Cumulus Encrypted Storage System) authors

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/CESSProject/cess-oss/configs"
)

var sendFileBufPool = &sync.Pool{
	New: func() interface{} {
		return make([]byte, configs.SIZE_1MiB)
	},
}

type MsgFile struct {
	Token    string `json:"token"`
	RootHash string `json:"roothash"`
	FileHash string `json:"filehash"`
	FileSize int64  `json:"filesize"`
	LastSize int64  `json:"lastsize"`
	LastFile bool   `json:"lastfile"`
	Data     []byte `json:"data"`
}

func FileReq(conn net.Conn, token, fid string, files []string, lastsize int64) error {
	var (
		err     error
		num     int
		tempBuf []byte
		msgHead IMessage
		fs      *os.File
		finfo   os.FileInfo
		message = MsgFile{
			Token:    token,
			RootHash: fid,
			FileHash: "",
			Data:     nil,
		}
		dp       = NewDataPack()
		headData = make([]byte, dp.GetHeadLen())
	)

	readBuf := sendFileBufPool.Get().([]byte)
	defer func() {
		sendFileBufPool.Put(readBuf)
	}()

	for i := 0; i < len(files); i++ {
		fs, err = os.Open(files[i])
		if err != nil {
			return err
		}
		finfo, err = fs.Stat()
		if err != nil {
			return err
		}
		message.FileHash = filepath.Base(files[i])
		message.FileSize = finfo.Size()
		message.LastFile = false
		if (i + 1) == len(files) {
			message.LastFile = true
			message.LastSize = lastsize
		}

		for {
			num, err = fs.Read(readBuf)
			if err != nil && err != io.EOF {
				return err
			}
			if num == 0 {
				break
			}
			num += num
			if message.LastFile && num >= int(lastsize) {
				bound := cap(readBuf) + int(lastsize) - num
				message.Data = readBuf[:bound]
			} else {
				message.Data = readBuf[:num]
			}

			tempBuf, err = json.Marshal(&message)
			if err != nil {
				return err
			}

			//send auth message
			tempBuf, _ = dp.Pack(NewMsgPackage(Msg_File, tempBuf))
			_, err = conn.Write(tempBuf)
			if err != nil {
				return err
			}

			//read head
			_, err = io.ReadFull(conn, headData)
			if err != nil {
				return err
			}

			msgHead, err = dp.Unpack(headData)
			if err != nil {
				return err
			}

			if msgHead.GetMsgID() == Msg_OK_FILE {
				break
			}

			if msgHead.GetMsgID() == Msg_OK {
				if message.LastFile && num >= int(lastsize) {
					return nil
				}
			}

			if msgHead.GetMsgID() != Msg_OK {
				return fmt.Errorf("send file error")
			}
		}
	}

	return err
}
