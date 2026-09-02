// Reached by path because a node_modules link puts blueclaw's Go files under
// web/, where `go vet ./...` refuses to walk them.
export * from '../../../../../../.dependency/blueclaw/protocol/src/index.ts';
