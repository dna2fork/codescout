package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/golang/protobuf/proto"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif_typed"

	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol/reader"
)

func main() {
	// parse file into LsifValues proto
	file := os.Args[1]
	fileReader, err := os.Open(file)
	if err != nil {
		panic(err)
	}
	if strings.HasSuffix(file, ".lsif-typed") {
		data, err := ioutil.ReadAll(fileReader)
		if err != nil {
			panic(err)
		}
		index := lsif_typed.Index{}
		err = proto.Unmarshal(data, &index)
		if err != nil {
			panic(errors.Wrapf(err, "failed to parse protobuf file '%s'", file))
		}
		els, err := reader.ConvertTypedIndexToGraphIndex(&index)
		if err != nil {
			panic(errors.Wrapf(err, "failed reader.ConvertTypedIndexToGraphIndex"))
		}
		err = reader.WriteNDJSON(reader.ElementsToEmptyInterfaces(els), os.Stdout)
		if err != nil {
			panic(err)
		}
	} else {
		panic(fmt.Sprintf("unexpected file format (must have extension .lsif-typed): %s\n", file))
	}
}
