package reader

import (
	"encoding/json"
	"io"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol/writer"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif_typed"
)

func ConvertTypedIndexToGraphIndex(index *lsif_typed.Index) ([]Element, error) {
	g := newGraph()

	if index.Metadata == nil {
		return nil, errors.New(".Metadata is nil")
	}
	if index.Metadata.ToolInfo == nil {
		return nil, errors.New(".Metadata.ToolInfo is nil")
	}

	positionEncoding := ""
	switch index.Metadata.TextDocumentEncoding {
	case lsif_typed.TextEncoding_UTF8:
		positionEncoding = "utf-8"
	case lsif_typed.TextEncoding_UTF16:
		positionEncoding = "utf-16"
	default:
		return nil, errors.New(".Metadata.TextDocumentEncoding does not have value utf-8 or utf-16")
	}

	g.emitVertex(
		"metaData",
		MetaData{
			Version:          "0.4.3", // Hardcoded LSIF Graph version.
			ProjectRoot:      index.Metadata.ProjectRoot,
			PositionEncoding: positionEncoding,
			ToolInfo: ToolInfo{
				Name:    index.Metadata.ToolInfo.Name,
				Version: index.Metadata.ToolInfo.Version,
			},
		},
	)

	// Pass 1: create result sets for global symbols.
	for _, importedSymbol := range index.ExternalSymbols {
		g.symbolToResultSet[importedSymbol.Symbol] = g.emitResultSet(importedSymbol, "import")
	}
	for _, document := range index.Documents {
		for _, exportedSymbol := range document.Symbols {
			g.registerInverseRelationships(exportedSymbol)
			if lsif_typed.IsGlobalSymbol(exportedSymbol.Symbol) {
				// Local symbols are skipped here because we handle them in the
				//second pass when processing individual documents.
				g.symbolToResultSet[exportedSymbol.Symbol] = g.emitResultSet(exportedSymbol, "export")
			}
		}
	}

	// Pass 2: emit ranges for all documents.
	for _, document := range index.Documents {
		g.emitDocument(index, document)
	}

	return g.Elements, nil
}

// graph is a helper struct to emit an LSIF Graph.
type graph struct {
	ID                   int
	Elements             []Element
	symbolToResultSet    map[string]symbolInformationIDs
	inverseRelationships map[string][]*lsif_typed.Relationship
	packageToGraphID     map[string]int
}

// symbolInformationIDs is a container for LSIF Graph IDs corresponding to an lsif_typed.SymbolInformation.
type symbolInformationIDs struct {
	ResultSet            int
	DefinitionResult     int
	ReferenceResult      int
	ImplementationResult int
	HoverResult          int
}

func newGraph() graph {
	return graph{
		ID:                   0,
		Elements:             []Element{},
		symbolToResultSet:    map[string]symbolInformationIDs{},
		packageToGraphID:     map[string]int{},
		inverseRelationships: map[string][]*lsif_typed.Relationship{},
	}
}

func (g *graph) emitPackage(pkg *lsif_typed.Package) int {
	id := pkg.ID()
	graphID, ok := g.packageToGraphID[id]
	if ok {
		return graphID
	}

	graphID = g.emitVertex("packageInformation", PackageInformation{
		Name:    pkg.Name,
		Version: pkg.Version,
	})
	g.packageToGraphID[pkg.ID()] = graphID
	return graphID
}

func (g *graph) emitResultSet(info *lsif_typed.SymbolInformation, monikerKind string) symbolInformationIDs {

	hover := strings.Join(info.Documentation, "\n\n---\n\n")
	definitionResult := -1
	isExported := monikerKind == "export"
	if isExported {
		definitionResult = g.emitVertex("definitionResult", nil)
	}
	//hasImplementationRelationship := false
	//for _, relationship := range info.Relationships {
	//	if relationship.IsImplementation {
	//		hasImplementationRelationship = true
	//		break
	//	}
	//}
	//if hasImplementationRelationship {
	//	implementationResult = g.emitVertex("implementationResult", nil)
	//}
	ids := symbolInformationIDs{
		ResultSet:            g.emitVertex("resultSet", ResultSet{}),
		DefinitionResult:     definitionResult,
		ReferenceResult:      g.emitVertex("referenceResult", nil),
		ImplementationResult: -1,
		HoverResult:          g.emitVertex("hoverResult", hover),
	}
	if isExported {
		g.emitEdge("textDocument/definition", Edge{OutV: ids.ResultSet, InV: ids.DefinitionResult})
	}
	//if hasImplementationRelationship {
	//	g.emitEdge("textDocument/implementation", Edge{OutV: ids.ResultSet, InV: ids.ImplementationResult})
	//}
	g.emitEdge("textDocument/references", Edge{OutV: ids.ResultSet, InV: ids.ReferenceResult})
	g.emitEdge("textDocument/hover", Edge{OutV: ids.ResultSet, InV: ids.HoverResult})
	if monikerKind == "export" || monikerKind == "import" {
		g.emitMoniker(info.Symbol, monikerKind, ids.ResultSet)
	}

	return ids
}

func (g *graph) registerInverseRelationships(info *lsif_typed.SymbolInformation) {
	for _, relationship := range info.Relationships {
		inverseRelationships, _ := g.inverseRelationships[relationship.Symbol]
		g.inverseRelationships[relationship.Symbol] = append(inverseRelationships, &lsif_typed.Relationship{
			Symbol:           info.Symbol,
			IsReference:      relationship.IsReference,
			IsImplementation: relationship.IsImplementation,
			IsTypeDefinition: relationship.IsTypeDefinition,
		})
	}
}

func (g *graph) emitDocument(index *lsif_typed.Index, doc *lsif_typed.Document) {
	uri := filepath.Join(index.Metadata.ProjectRoot, doc.RelativePath)
	documentID := g.emitVertex("document", uri)
	localResultIDs := map[string]symbolInformationIDs{}
	symtab := map[string]*lsif_typed.SymbolInformation{}
	for _, info := range doc.Symbols {
		symtab[info.Symbol] = info
		if lsif_typed.IsLocalSymbol(info.Symbol) {
			localResultIDs[info.Symbol] = g.emitResultSet(info, "")
		}
		for _, relationship := range info.Relationships {
			if relationship.IsImplementation {
				relationshipIDs, ok := g.infoIDs(relationship.Symbol, localResultIDs)
				if ok && relationshipIDs.DefinitionResult > 0 {
					// Not an imported symbol
					continue
				}
				infoIDs, ok := g.infoIDs(info.Symbol, localResultIDs)
				if !ok {
					continue
				}
				g.emitMoniker(relationship.Symbol, "implementation", infoIDs.ResultSet)
			}
		}
	}

	var rangeIDs []int
	for _, occ := range doc.Occurrences {
		rangeID, err := g.emitRange(occ.Range)
		if err != nil {
			// Silently skip invalid ranges.
			continue
		}
		rangeIDs = append(rangeIDs, rangeID)
		resultIDs, ok := g.infoIDs(occ.Symbol, localResultIDs)
		if !ok {
			// Silently skip occurrences to symbols with no matching SymbolInformation.
			continue
		}
		g.emitEdge("next", Edge{OutV: rangeID, InV: resultIDs.ResultSet})
		isDefinition := occ.SymbolRoles&int32(lsif_typed.SymbolRole_Definition) != 0
		if isDefinition && resultIDs.DefinitionResult > 0 {
			g.emitEdge("item", Edge{OutV: resultIDs.DefinitionResult, InVs: []int{rangeID}, Document: documentID})
			symbolInfo, ok := symtab[occ.Symbol]
			if ok {
				g.emitReferenceResults(rangeID, documentID, resultIDs, localResultIDs, symbolInfo)
			}
		} else { // reference
			g.emitEdge("item", Edge{OutV: resultIDs.ReferenceResult, InVs: []int{rangeID}, Document: documentID})
		}
	}
	g.emitEdge("contains", Edge{OutV: documentID, InVs: rangeIDs})
}

func (g *graph) emitReferenceResults(rangeID, documentID int, resultIDs symbolInformationIDs, localResultIDs map[string]symbolInformationIDs, info *lsif_typed.SymbolInformation) {
	var allReferenceResultIds []int
	relationships, _ := g.inverseRelationships[info.Symbol]
	for _, relationship := range relationships {
		allReferenceResultIds = append(allReferenceResultIds, g.emitRelationship(relationship, rangeID, documentID, localResultIDs)...)
	}
	for _, relationship := range info.Relationships {
		allReferenceResultIds = append(allReferenceResultIds, g.emitRelationship(relationship, rangeID, documentID, localResultIDs)...)
	}
	if len(allReferenceResultIds) > 0 {
		g.emitEdge("item", Edge{
			OutV:     resultIDs.ReferenceResult,
			InVs:     allReferenceResultIds,
			Document: documentID,
			// The 'property' field is included in the LSIF Graph JSON but it's not present in reader.Element
			// Property: "referenceResults",
		})
	}
}

func (g *graph) emitRelationship(relationship *lsif_typed.Relationship, rangeID, documentID int, localResultIDs map[string]symbolInformationIDs) []int {
	relationshipIDs, ok := g.infoIDs(relationship.Symbol, localResultIDs)
	if !ok {
		return nil
	}

	if relationship.IsImplementation {
		if relationshipIDs.ImplementationResult < 0 {
			relationshipIDs.ImplementationResult = g.emitVertex("implementationResult", nil)
			g.emitEdge("textDocument/implementation", Edge{OutV: relationshipIDs.ResultSet, InV: relationshipIDs.ImplementationResult})
		}
		g.emitEdge("item", Edge{OutV: relationshipIDs.ImplementationResult, InVs: []int{rangeID}, Document: documentID})
	}

	if relationship.IsReference {
		g.emitEdge("item", Edge{
			OutV:     relationshipIDs.ReferenceResult,
			InVs:     []int{rangeID},
			Document: documentID,
			// The 'property' field is included in the LSIF Graph JSON but it's not present in reader.Element
			// Property: "referenceResults",
		})
		return []int{relationshipIDs.ReferenceResult}
	}

	return nil
}

func (g *graph) emitMoniker(symbolID string, kind string, resultSetID int) {
	symbol, err := lsif_typed.ParsePartialSymbol(symbolID, false)
	if err == nil && symbol != nil && symbol.Scheme != "" {
		// Accept the symbol as long as it has a non-empty scheme. We ignore
		// parse errors because we can still provide accurate
		// definition/references/hover within a repo.
		monikerID := g.emitVertex("moniker", Moniker{
			Kind:       kind,
			Scheme:     symbol.Scheme,
			Identifier: symbolID,
		})
		g.emitEdge("moniker", Edge{OutV: resultSetID, InV: monikerID})
		if symbol.Package != nil &&
			symbol.Package.Manager != "" &&
			symbol.Package.Name != "" &&
			symbol.Package.Version != "" {
			packageID := g.emitPackage(symbol.Package)
			g.emitEdge("packageInformation", Edge{OutV: monikerID, InV: packageID})
		}
	}
}

func (g *graph) emitRange(lsifRange []int32) (int, error) {
	startLine, startCharacter, endLine, endCharacter, err := interpretLsifRange(lsifRange)
	if err != nil {
		return 0, err
	}
	return g.emit("vertex", "range", Range{
		RangeData: protocol.RangeData{
			Start: protocol.Pos{
				Line:      int(startLine),
				Character: int(startCharacter),
			},
			End: protocol.Pos{
				Line:      int(endLine),
				Character: int(endCharacter),
			},
		},
	}), nil
}

func (g *graph) emitVertex(label string, payload interface{}) int {
	return g.emit("vertex", label, payload)
}

func (g *graph) emitEdge(label string, payload Edge) {
	if payload.InV == 0 && len(payload.InVs) == 0 {
		panic("no inVs")
	}
	g.emit("edge", label, payload)
}

func (g *graph) emit(ty, label string, payload interface{}) int {
	g.ID++
	g.Elements = append(g.Elements, Element{
		ID:      g.ID,
		Type:    ty,
		Label:   label,
		Payload: payload,
	})
	return g.ID
}

func interpretLsifRange(rnge []int32) (startLine, startCharacter, endLine, endCharacter int32, err error) {
	if len(rnge) == 3 {
		return rnge[0], rnge[1], rnge[0], rnge[2], nil
	}
	if len(rnge) == 4 {
		return rnge[0], rnge[1], rnge[2], rnge[3], nil
	}
	return 0, 0, 0, 0, errors.Newf("invalid LSIF range %v", rnge)
}

func (g *graph) infoIDs(symbol string, localSymtab map[string]symbolInformationIDs) (symbolInformationIDs, bool) {
	symtab := g.symbolToResultSet
	if lsif_typed.IsLocalSymbol(symbol) {
		symtab = localSymtab
	}
	ids, ok := symtab[symbol]
	return ids, ok
}

func WriteNDJSON(elements []interface{}, out io.Writer) error {
	w := writer.NewJSONWriter(out)
	for _, e := range elements {
		w.Write(e)
	}
	return w.Flush()
}

func ElementsToEmptyInterfaces(els []Element) []interface{} {
	var r []interface{}
	for _, el := range els {
		object := map[string]interface{}{
			"id":    el.ID,
			"type":  el.Type,
			"label": el.Label,
		}
		switch el.Label {
		case "hoverResult":
			object["result"] = map[string]interface{}{
				"contents": map[string]interface{}{
					"kind":  "markdown",
					"value": el.Payload,
				},
			}
		case "document":
			object["uri"] = el.Payload
		default:
			if el.Payload != nil {
				payload, err := json.Marshal(el.Payload)
				if err != nil {
					panic(err)
				}
				var x interface{}
				err = json.Unmarshal(payload, &x)
				if err != nil {
					panic(err)
				}
				if x != nil {
					for key, value := range x.(map[string]interface{}) {
						object[key] = value
					}
				}
			}
		}
		r = append(r, object)
	}
	return r
}
