/*
	Copyright (C) CESS. All rights reserved.
	Copyright (C) Cumulus Encrypted Storage System. All rights reserved.

	SPDX-License-Identifier: Apache-2.0
*/

package chain

import (
	"fmt"
	"log"

	"github.com/CESSProject/DeOSS/pkg/utils"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/xxhash"
	"github.com/pkg/errors"
)

// GetPublicKey returns your own public key
func (c *chainClient) GetPublicKey() []byte {
	return c.keyring.PublicKey
}

func (c *chainClient) GetMnemonicSeed() string {
	return c.keyring.URI
}

func (c *chainClient) GetSyncStatus() (bool, error) {
	if !c.IsChainClientOk() {
		return false, ERR_RPC_CONNECTION
	}
	h, err := c.api.RPC.System.Health()
	if err != nil {
		return false, err
	}
	return h.IsSyncing, nil
}

func (c *chainClient) GetChainStatus() bool {
	return c.GetChainState()
}

// Get miner information on the chain
func (c *chainClient) GetStorageMinerInfo(pkey []byte) (MinerInfo, error) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(utils.RecoverError(err))
		}
	}()
	var data MinerInfo

	if !c.IsChainClientOk() {
		c.SetChainState(false)
		return data, ERR_RPC_CONNECTION
	}
	c.SetChainState(true)

	key, err := types.CreateStorageKey(
		c.metadata,
		SMINER,
		MINERITEMS,
		pkey,
	)
	if err != nil {
		return data, errors.Wrap(err, "[CreateStorageKey]")
	}

	ok, err := c.api.RPC.State.GetStorageLatest(key, &data)
	if err != nil {
		return data, errors.Wrap(err, "[GetStorageLatest]")
	}
	if !ok {
		return data, ERR_RPC_EMPTY_VALUE
	}
	return data, nil
}

// Get all miner information on the cess chain
func (c *chainClient) GetAllStorageMiner() ([]types.AccountID, error) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(utils.RecoverError(err))
		}
	}()
	var data []types.AccountID

	if !c.IsChainClientOk() {
		c.SetChainState(false)
		return data, ERR_RPC_CONNECTION
	}
	c.SetChainState(true)

	key, err := types.CreateStorageKey(
		c.metadata,
		SMINER,
		ALLMINER,
	)
	if err != nil {
		return nil, errors.Wrap(err, "[CreateStorageKey]")
	}

	ok, err := c.api.RPC.State.GetStorageLatest(key, &data)
	if err != nil {
		return nil, errors.Wrap(err, "[GetStorageLatest]")
	}
	if !ok {
		return nil, ERR_RPC_EMPTY_VALUE
	}
	return data, nil
}

// Query file meta info
func (c *chainClient) GetFileMetaInfo(fid string) (FileMetaInfo, error) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(utils.RecoverError(err))
		}
	}()
	var (
		data FileMetaInfo
		hash FileHash
	)

	if !c.IsChainClientOk() {
		c.SetChainState(false)
		return data, ERR_RPC_CONNECTION
	}
	c.SetChainState(true)

	if len(hash) != len(fid) {
		return data, errors.New("invalid filehash")
	}

	for i := 0; i < len(hash); i++ {
		hash[i] = types.U8(fid[i])
	}

	b, err := types.Encode(hash)
	if err != nil {
		return data, errors.Wrap(err, "[Encode]")
	}

	key, err := types.CreateStorageKey(
		c.metadata,
		FILEBANK,
		FILE,
		b,
	)
	if err != nil {
		return data, errors.Wrap(err, "[CreateStorageKey]")
	}

	ok, err := c.api.RPC.State.GetStorageLatest(key, &data)
	if err != nil {
		return data, errors.Wrap(err, "[GetStorageLatest]")
	}
	if !ok {
		return data, ERR_RPC_EMPTY_VALUE
	}
	return data, nil
}

func (c *chainClient) GetCessAccount() (string, error) {
	return utils.EncodePublicKeyAsCessAccount(c.keyring.PublicKey)
}

func (c *chainClient) GetAccountInfo(pkey []byte) (types.AccountInfo, error) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(utils.RecoverError(err))
		}
	}()
	var data types.AccountInfo

	if !c.IsChainClientOk() {
		c.SetChainState(false)
		return data, ERR_RPC_CONNECTION
	}
	c.SetChainState(true)

	b, err := types.Encode(types.NewAccountID(pkey))
	if err != nil {
		return data, errors.Wrap(err, "[EncodeToBytes]")
	}

	key, err := types.CreateStorageKey(
		c.metadata,
		SYSTEM,
		ACCOUNT,
		b,
	)
	if err != nil {
		return data, errors.Wrap(err, "[CreateStorageKey]")
	}

	ok, err := c.api.RPC.State.GetStorageLatest(key, &data)
	if err != nil {
		return data, errors.Wrap(err, "[GetStorageLatest]")
	}
	if !ok {
		return data, ERR_RPC_EMPTY_VALUE
	}
	return data, nil
}

func (c *chainClient) GetState(pubkey []byte) (string, error) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(utils.RecoverError(err))
		}
	}()
	var data Ipv4Type

	if !c.IsChainClientOk() {
		c.SetChainState(false)
		return "", ERR_RPC_CONNECTION
	}
	c.SetChainState(true)

	b, err := types.Encode(types.NewAccountID(pubkey))
	if err != nil {
		return "", errors.Wrap(err, "[EncodeToBytes]")
	}

	key, err := types.CreateStorageKey(
		c.metadata,
		OSS,
		OSS,
		b,
	)
	if err != nil {
		return "", errors.Wrap(err, "[CreateStorageKey]")
	}

	ok, err := c.api.RPC.State.GetStorageLatest(key, &data)
	if err != nil {
		return "", errors.Wrap(err, "[GetStorageLatest]")
	}
	if !ok {
		return "", ERR_RPC_EMPTY_VALUE
	}

	return fmt.Sprintf("%d.%d.%d.%d:%d",
		data.Value[0],
		data.Value[1],
		data.Value[2],
		data.Value[3],
		data.Port), nil
}

func (c *chainClient) GetGrantor(pkey []byte) (types.AccountID, error) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(utils.RecoverError(err))
		}
	}()
	var data types.AccountID

	if !c.IsChainClientOk() {
		c.SetChainState(false)
		return data, ERR_RPC_CONNECTION
	}
	c.SetChainState(true)

	b, err := types.Encode(types.NewAccountID(pkey))
	if err != nil {
		return data, errors.Wrap(err, "[EncodeToBytes]")
	}

	key, err := types.CreateStorageKey(
		c.metadata,
		OSS,
		AUTHORITYLIST,
		b,
	)
	if err != nil {
		return data, errors.Wrap(err, "[CreateStorageKey]")
	}

	ok, err := c.api.RPC.State.GetStorageLatest(key, &data)
	if err != nil {
		return data, errors.Wrap(err, "[GetStorageLatest]")
	}
	if !ok {
		return data, ERR_RPC_EMPTY_VALUE
	}
	return data, nil
}

func (c *chainClient) GetBucketInfo(owner_pkey []byte, name string) (BucketInfo, error) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(utils.RecoverError(err))
		}
	}()
	var data BucketInfo

	if !c.IsChainClientOk() {
		c.SetChainState(false)
		return data, ERR_RPC_CONNECTION
	}
	c.SetChainState(true)

	owner, err := types.Encode(types.NewAccountID(owner_pkey))
	if err != nil {
		return data, errors.Wrap(err, "[EncodeToBytes]")
	}

	name_byte, err := types.Encode(name)
	if err != nil {
		return data, errors.Wrap(err, "[Encode]")
	}

	key, err := types.CreateStorageKey(
		c.metadata,
		FILEBANK,
		BUCKET,
		owner,
		name_byte,
	)
	if err != nil {
		return data, errors.Wrap(err, "[CreateStorageKey]")
	}

	ok, err := c.api.RPC.State.GetStorageLatest(key, &data)
	if err != nil {
		return data, errors.Wrap(err, "[GetStorageLatest]")
	}
	if !ok {
		return data, ERR_RPC_EMPTY_VALUE
	}
	return data, nil
}

func (c *chainClient) GetBucketList(owner_pkey []byte) ([]types.Bytes, error) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(utils.RecoverError(err))
		}
	}()
	var data []types.Bytes

	if !c.IsChainClientOk() {
		c.SetChainState(false)
		return data, ERR_RPC_CONNECTION
	}
	c.SetChainState(true)

	owner, err := types.Encode(types.NewAccountID(owner_pkey))
	if err != nil {
		return data, errors.Wrap(err, "[EncodeToBytes]")
	}

	key, err := types.CreateStorageKey(
		c.metadata,
		FILEBANK,
		BUCKETLIST,
		owner,
	)
	if err != nil {
		return data, errors.Wrap(err, "[CreateStorageKey]")
	}

	ok, err := c.api.RPC.State.GetStorageLatest(key, &data)
	if err != nil {
		return data, errors.Wrap(err, "[GetStorageLatest]")
	}
	if !ok {
		return data, ERR_RPC_EMPTY_VALUE
	}
	return data, nil
}

// Get scheduler information on the cess chain
func (c *chainClient) GetSchedulerList() ([]SchedulerInfo, error) {
	c.lock.Lock()
	defer func() {
		c.lock.Unlock()
		if err := recover(); err != nil {
			//fmt.Println(utils.RecoverError(err))
		}
	}()
	var data []SchedulerInfo

	if !c.IsChainClientOk() {
		c.SetChainState(false)
		return data, ERR_RPC_CONNECTION
	}
	c.SetChainState(true)

	key, err := types.CreateStorageKey(
		c.metadata,
		TEEWORKER,
		SCHEDULERMAP,
	)
	if err != nil {
		return data, errors.Wrap(err, "[CreateStorageKey]")
	}

	ok, err := c.api.RPC.State.GetStorageLatest(key, &data)
	if err != nil {
		return data, errors.Wrap(err, "[GetStorageLatest]")
	}
	if !ok {
		return data, ERR_RPC_EMPTY_VALUE
	}
	return data, nil
}

// Pallert
const (
	_FILEBANK = "FileBank"
	_SYSTEM   = "System"
	_CACHER   = "Cacher"
)

// Chain state
const (
	// System
	_SYSTEM_ACCOUNT = "Account"
	_SYSTEM_EVENTS  = "Events"
	// FileMap
	_FILEMAP_FILEMETA = "File"
	// Miner
	_CACHER_CACHER = "Cachers"
)

type CacherInfo struct {
	Acc       types.AccountID
	Ip        Ipv4Type
	BytePrice types.U128
}

func (c *chainClient) GetCachers() ([]CacherInfo, error) {
	var list []CacherInfo
	key := createPrefixedKey(_CACHER_CACHER, _CACHER)
	keys, err := c.api.RPC.State.GetKeysLatest(key)
	if err != nil {
		return list, errors.Wrap(err, "get cachers info error")
	}
	set, err := c.api.RPC.State.QueryStorageAtLatest(keys)
	if err != nil {
		return list, errors.Wrap(err, "get cachers info error")
	}
	for _, elem := range set {
		for _, change := range elem.Changes {
			var cacher CacherInfo
			if err := types.Decode(change.StorageData, &cacher); err != nil {
				//logger.Uld.Sugar().Error("get cachers info error,hash:", err)
				log.Println(err)
				continue
			}
			list = append(list, cacher)
		}
	}
	return list, nil
}

func createPrefixedKey(method, prefix string) []byte {
	return append(xxhash.New128([]byte(prefix)).Sum(nil), xxhash.New128([]byte(method)).Sum(nil)...)
}
