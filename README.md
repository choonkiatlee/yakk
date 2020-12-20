# Running the project.

### Run the signalling server
From the yakk directory, run: 
```go
go run ./cmd/yakkserver/.
```

### Run the yakk server
From the yakk directory, run 
```bash
go run ./cmd/yakk/. -l 8080
```

### Run the yakk client
From the yakk directory, run 
```bash
go run ./cmd/yakk/. client -l 8080
```
