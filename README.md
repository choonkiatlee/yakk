# YAKK -- "Anything" over WebRTC

Ever wanted to run ssh over a p2p connection? Or share a development webserver with friends but don't have a public IP address? 
Yakk has got you covered! 

YAKK is a simple set of go programs that creates a p2p "tunnel" between computers, allowing you to share
traffic between the connected computers without knowing each other's public IP addresses.

# Quickstart Example 1:

Alice wants to share an ssh server that she has running on her localhost port 22 with Bob, who is inconveniently on the other side
of the world and is unable to get Alice's public IP to connect. What then? 

We can use YAKK to create a tunnel between Bob's port `9000` and Alice's port `22` using webrtc, allowing Alice and Bob to connect to each other.

Alice runs: 
```bash
./yakk server -p 22
> Your mailroom name is: swimming
> Input MailRoom PW:
> thisisanotsostrongpassword
>
> One line connection command: yakk client -p <port> swimming --pw thisisanotsostrongpassword
```

Bob runs:
```bash
./yakk client -p 9000 swimming --pw thisisanotsostrongpassword
ssh bob@localhost:9000   # et voila! ssh'ed into Alice's computer :)
```

# Quickstart Example 2:

Alice wants to share a file with Bob. Because of how general YAKK is, this is super simple. Leveraging on the above, we create a 
webrtc tunnel between Alice and Bob, then serve a simple http server on Alice's end which serves the file. Bob can then connect
to the http server to download the required file. 

Alice runs: 
```bash
./yakk filesend path/to/file
> Your mailroom name is: swimming
> Input MailRoom PW:
> thisisanotsostrongpassword
>
> One line connection command: yakk filereceive swimming --pw thisisanotsostrongpassword
```

Bob runs:
```bash
./yakk filereceive swimming --pw thisisanotsostrongpassword
> Downloaded path/to/file!
```

# How it works (the gory details): 

The first line, `./yakk server -p 22` opens a websocket connection to a yakk signalling server. The yakk signalling server
replies and allocates Alice a `mailroom`. Alice then sets a `mailroom password`. This is a simple low entropy password that 
Alice and Bob can use to authenticate themselves. 

When Bob runs `./yakk client -p 9000 swimming --pw asdasd`, Bob also opens a websocket connection to the same yakk signalling
server, and enters the mailroom created by Alice. Using the shared low entropy password, Alice and Bob exchange the password
for a shared high security cryptographic key using [Password Authenticated Key Exchange by Juggling (J-PAKE)](https://en.wikipedia.org/wiki/Password_Authenticated_Key_Exchange_by_Juggling) (implementation details [here](https://choonkiatlee.github.io/jpake-implementation/)).

This high security cryptographic key is then used to encrypt the webrtc signalling information to be exchanged, thus preventing potential leakage
of sensitive connection information by Alice and Bob. The use of J-PAKE also means that Alice and Bob both DO NOT have to trust the signalling
server! Any plain-ol' server would work.

On Bob's side, after the encrypted webrtc datachannel is negotiated, the yakk program listens on the specified port `9000`. When Bob then 
starts an ssh connection to his localhost port `9000`, the listening yakk program pipes the traffic down port `bob's port 9000` through the 
encrypted datachannel to Alice's yakk program, which connects to `alice's port 22` and replays this traffic there. 

This means that effectively, we have created a tunnel between Bob's port `9000` and Alice's port `22` through webrtc! 

```
+----------------------------+                    +----------------------------+
|                            |                    |                            |
| Alice                      |                    | Bob                        |
|                            |                    |                            |
| SSH Server on Port 22      |                    | SSH Client connects to     |
|                            |                    | localhost port 22          |
+-------------+--------------+                    +--------------+-------------+
              |                                                  |
              v                                                  v
+-------------+--------------+                    +--------------+-------------+
|                            |                    |                            |
| Alice                      |                    | Bob                        |
|                            |                    |                            |
| Connect to Port 22         |                    | Listen on localhost port 22|
|                            |                    |                            |
+-------------+--------------+                    +--------------+-------------+
              |                                                  |
              v                                                  v
+-------------+--------------+                    +--------------+-------------+
|                            |                    |                            |
| Alice                      |                    | Bob                        |
|                            +<------------------>+                            |
| WebRTC DataChannel         |                    | WebRTC DataChannel         |
|                            |                    |                            |
+----------------------------+                    +----------------------------+

              +--------------------------------------------------+
              |                                                  |
              |   Connection negotiated using the YAKK           |
              |   Signalling Server. Fully end-to-end encrypted  |
              |   using PAKE/TLS for security                    |
              |                                                  |
              +--------------------------------------------------+

```


## Running the project for development
### Run the signalling server
From the yakk directory, run: 
```go
go run ./cmd/yakkserver/.
```

### Run the yakk server
From the yakk directory, run 
```bash
go run ./cmd/yakk/. -p 8080
```

### Run the yakk client
From the yakk directory, run 
```bash
go run ./cmd/yakk/. client -p 8080
```

# Doc Links:
## Edit Ascii Art
- [PAKE Procedure](http://stable.ascii-flow.appspot.com/#2326929467921821744/401694744)
- [WebRTC Connection Procedure](http://stable.ascii-flow.appspot.com/#2741619146174214850/79893535)


