# WhatevrMe

## Run

`cd`
`./run.sh`

## Create a Note

`curl -H "Content-Type: application/json" -X POST -d '{"timestamp":1503898902398,"cipher_text":null}' http://127.0.0.1:8880/api/note`

Check the note's HTML page: `http://127.0.0.1:8880/h-gKZ_x7DVFUFC3NPZ1kQj6p7kCglSOhRXO2wWM9QRg`
Or fetch it's JSON via REST API: `http://127.0.0.1:8880/api/note/h-gKZ_x7DVFUFC3NPZ1kQj6p7kCglSOhRXO2wWM9QRg`
