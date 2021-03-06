/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at
  http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/

package main

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"hyperledger/cci/appinit"
	"hyperledger/cci/org/hyperledger/chaincode/example02"
	"hyperledger/ccs"

	"encoding/json"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"crypto/sha256"
)

var logger = shim.NewLogger("mylogger")

const oneDayUnixTime int = 86400*(10**9)
const oneYearUnixTime int = 365*86400*(10**9)

type AerialCC struct {
	name string
	symbol string
	decimals int

	chainStartTime int
	chainStartBlockNumber int
	stakeStartTime int
	stakeMinAge int
	stakeMaxAge int
	maxMineProofOfStake int

	totalSupply int
	maxTotalSupply int
	totalInitialSupply int

}

type TransferInStruct struct {
	Address string "json:address"
	Amount int "json:amount"
	Time int "json:time"
}
type transferIns []TransferInstruct


// Called to initialize the chaincode
func (t *AerialCC) Init(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {

	var err error

	logger.Info("Starting Initializing the Chaincode")

	if len(args) < 12 {
		logger.Error("Invalid number of arguments")
		return nil, error.New("Invalid number of arguments")
	}

	t.name = args["name"]
	t.symbol = args["symbol"]
	t.decimals = int32(args["decimal"])
	//Timings
	chainStartTime := int32(args["chainStartTime"])
	stakeStartTime := int32(args["stakeStarttime"])
	const shortForm = "2006-Jan-02"
	f, _ = time.Parse(shortForm, chainStartTime)
	g, _ = time.Parse(shortForm, stakeStartTime)
	t.chainStartTime = f.Unix()
	t.stakeStartTime = g.Unix()

	t.chainStartBlockNumber = int32(args["chainStartBlockNumber"])
	t.stakeMinAge = int32(args["stakeMinAge"])*oneDayUnixTime
	t.stakeMaxAge = int32(args["stakeMaxAge"])*oneDayUnixTime
	t.maxMineProofOfStake = args["maxMineProofOfStake"]

	t.totalSupply = args["totalSupply"]
	t.maxTotalSupply = args["maxTotalSupply"]
	t.totalInitialSupply = args["totalInitialSupply"]

	logger.Info("Successfully Initialized the AerialCC")

	return nil, nil

}

func (t *AerialCC) Query(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	if function == "MakePayment" {
		return MakePayment(stub, args)
	} else if function == "DeleteAccount" {
		return DeleteAccount(stub, args)
		} else if function == "CheckBalance" {
			return CheckBalance(stub, args)
		}
	return nil, nil
}

// Transaction makes payment of X units from A to B
func MakePayment(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {

	var err error

	src, err := stub.GetState(stub, args["partySrc"])
	if err != nil {
		logger.Error("partySrc is missing!")
		return nil, err
	}

	dst, err := stub.GetState(stub, args["partyDst"])
	if err != nil {
		logger.Error("partyDst is missing!")
		return nil, err
	}

	X := int(param.Amount)
	src = src - X
	dst = dst + X
	logger.Info("srcAmount = %d, dstAmount = %d\n", src, dst)

	err = stub.PutState(args["partySrc"], []byte(strconv.Itoa(src)))
	if err != nil {
		logger.Error("failed to write the state for src!")
		return nil, err
	}

	err = stub.PutState(args["partyDst"], []byte(strconv.Itoa(dst)))
	if err != nil {
		logger.Error("failed to write the state for dst!")
		return nil, err
	}

	return nil, nil
}

// Deletes an entity from state
func DeleteAccount(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {

	err := stub.DelState(args["partyID"])
	if err != nil {
		logger.Error("Failed to delete state!")
		return nil, errors.New("Failed to delete state")
	}

	return nil, nil

}

// Query callback representing the query of a chaincode
func CheckBalance(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	var err error

	val, err := stub.GetState(stub, args["partyID"])
	if err != nil {
		return nil, err
	}
	logger.Info("Query Response: %d\n", val)
	return val, nil
}

func MinePoS(stub shim.ChaincodeStubInterface, args []string) (bool,error) {

	//canPoSMint
	src, err := t.GetState(stub, param.PartySrc)
	if err != nil {
		return false, err
	}

	st := string(append(param.PartySrc,"transferIn"))
	transferinsID := sha256.New()
	transferinsID.Write([]byte (st))
	transferIns, err := stub.GetState(transferinsID.Sum(nil))
	var um transferIns
	err = json.Unmarshal(transferIns, &um)

	if err != nil {
		return false, err
	}

	if len(transferIns) <= 0 {
		return false, err
	}

	reward := t.getProofOfStakeReward(stub, param.PartySrc)
	if reward <= 0 {
		return false, err
	}

	newTS, err := t.increaseTotalSupply(reward)
	if err != nil {
		fmt.Printf("IncreaseTotalSupply Failed: %s", err)
		return false, err
	}

	src = src + reward
	err = stub.PutState(param.PartySrc, []byte(strconv.Itoa(src)))
	if err != nil {
		return false, err
	}

	um = nil
	temp_tin := transferInStruct({"Address":param.PartySrc,"Amount":src+reward,"Time":time.Now().Unix()})
	um = append(um, temp_tin)
	um, err = json.Marshal(&um)
	if err != nil {
		return false, err
	}
	stub.PutState(transferinsID.Sum(), um)

	return true, nil
}

func getProofOfStakeReward(stub shim.ChaincodeStubInterface, args []string) (int, bool) {

	now := time.Now().Unix()
	if now <= t.stakeStartTime || stakeStartTime <= 0 {
		return 0,false
	}

	_coinAge = getCoinAge(stub, param, now)
	if _coinAge <= 0 {
		return 0, false
	}

	var interest int
	interest = t.maxMintProofOfStake
	if (now - t.stakeStartTime) / oneYearUnixTime == 0 {
		interest = (770 * t.maxMintProofOfStake) / 100
	} else if (now - t.stakeStartTime) / oneYearUnixTime == 1 {
		interest = (435 * maxMintProofOfStake) / 100
	}

	return (_coinAge * interest) / (365* (10**t.decimals)), true

}

func getCoinAge(stub shim.ChaincodeStubInterface, now time, function string, args []string) (int, bool) {

	st := string(append(param.PartySrc,"transferIn"))
	transferinsID := sha256.New()
	transferinsID.Write([]byte (st))
	transferIns, err := stub.GetState(transferinsID.Sum(nil))
	var um transferIns
	err = json.Unmarshal(transferIns, &um)

	if err != nil {
		return 0, false
	}

	if len(transferIns) <= 0 {
		return 0, false
	}

	var _coinAge int
	for i := 0, i < len(transferIns); i++ {
		if now.Unix() < (transferIns[i].Time + t.stakeMinAge){
			continue
		}
		var nCoinSeconds int
		nCoinSeconds = now.Unix - transferIns[i].Time
		if nCoinSeconds > t.stakeMaxAge {
			nCoinSeconds = t.stakeMaxAge
		}
		_coinAge = _coinAge + transferIns[i].Amount * (nCoinSeconds / 86400*(10**9))
	}
	return _coinAge, true
}

func main() {

	lld, _ := shim.LogLevel("DEBUG")
	fmt.Println(lld)

	logger.SetLevel(lld)
	fmt.Println(logger.IsEnabledFor(lld))

	err := shim.Start(new(AerialCC))
	if err != nil {
		logger.Error("Could not start AerialCC")
	} else {
		logger.Info("AerialCC successfully started")
	}

}
