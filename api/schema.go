// GENERATED from sourcegraph.schema - DO NOT EDIT

package api

var Schema = `schema {
	query: Query
}

interface Node {
	id: ID!
}

type Query {
	root: Root!
	node(id: ID!): Node
}

type Root {
	repository(uri: String!): Repository
	remoteRepositories: [RemoteRepository!]!
	remoteStarredRepositories: [RemoteRepository!]!
}

type RefFields {
	refLocation: RefLocation
	uri: URI
}

type URI {
	host: String!
	fragment: String!
	path: String!
	query: String!
	scheme: String!
}

type RefLocation {
	startLineNumber: Int!
	startColumn: Int!
	endLineNumber: Int!
	endColumn: Int!
}

type Repository implements Node {
	id: ID!
	uri: String!
	description: String!
	commit(rev: String!): CommitState!
	latest: CommitState!
	defaultBranch: String!
	branches: [String!]!
	tags: [String!]!
	contributors: [Contributor!]!
}

type Contributor {
	login:             String!
	avatarURL:         String!
	contributions:     Int!
}

type CommitState {
	commit: Commit
	cloneInProgress: Boolean!
}

type Commit implements Node {
	id: ID!
	sha1: String!
	tree(path: String = "", recursive: Boolean = false): Tree
	file(path: String!): File
	languages: [String!]!
}

type Tree {
	directories: [Directory]!
	files: [File]!
}

type Directory {
	name: String!
	tree: Tree!
}

type File {
	name: String!
	content: String!
	blame(startLine: Int!, endLine: Int!): [Hunk!]!
	definition(line: Int!, column: Int!, language: String!): Definition
}

type Definition {
   globalReferences: [RefFields!]!
}

type RemoteRepository {
	uri: String!
	description: String!
	owner: String!
	name: String!
	httpCloneURL: String!
	language: String!
	fork: Boolean!
	mirror: Boolean!
	private: Boolean!
	createdAt: String!
	pushedAt: String!
	vcsSyncedAt: String!
	contributors: [Contributor!]!
}

type Hunk {
	startLine: Int!
	endLine: Int!
	startByte: Int!
	endByte: Int!
	rev: String!
	name: String!
	email: String!
	date: String!
	message: String!
	gravatarHash: String!
}
`
