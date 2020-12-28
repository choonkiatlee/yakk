# Misc Implementation Details

## Signalling

### 1. Room Creation / Joining

                       +                        +
      Callee           |       Server           |    Caller
+----------------------|------------------------|-----------------+
                       |                        |
        Create Room ---|---> Generate unique ---|
                       |     RoomID             |
                       |                        |
                     RoomID                     |
  Input Low Entropy<---|---------               |
  Shared Password      |                        |
                       |                        |
               Share Password/RoomID via other medium
                   --------------------------------->  Join Room
                       |                        |
                       |                        |
                       | Check if room exists---|----  Join Room
                       | If it exists, staple   |
                       | WS comms together      |
                       |                        |
                       +                        +


### PAKE Exchange Procedure

For JPAKE, see: [this post](https://choonkiatlee.github.io/jpake-implementation/) and [this repo](https://github.com/choonkiatlee/jpake-go) for details on the exact procedure.

### WebRTC Connection through the YAKK Signalling Server
Built according to [MDN Docs](https://developer.mozilla.org/en-US/docs/Web/API/WebRTC_API/Signaling_and_video_calling)

                                                +
                 Caller                         |                      Callee
+-----------------------------------------------|------------------------------------------+
        |                                       |
 EXCHAN |                InitPeerConnection     |
 GING   |             InitDataChannelCaller     |
 PAKE   |      HandleNegotiationNeededEvent     |
        |                     YAKKMSG_OFFER ----|---> InitPeerConnection
        |                                       |     InitDataChannelCallee
 WAIT   | (Begin Send/Recv of ICE Candidates)   |     HandleOfferMsg
 FOR    |                                       |
 ANSWER |                    HandleAnswerMsg<---|---- YAKKMSG_ANSWER
        |                                       |
        |                                       |     (Begin Send/Recv of ICE Candidates)
        |                                       |
        +
