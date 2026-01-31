const setupWs = (uri) => {
  try {
  ws = new WebSocket(uri);
  } catch (e) {
      console.log("failed connecting the websocket")
      return
  }

  ws.onopen = function() {
    console.log('Connected');
  }

  ws.onmessage = function(evt) {
    let d = evt.data
    try {
      d = JSON.parse(evt.data)
    } catch (e) {
      console.log("failed to parse message from server")
      return
    }
    let out
    let ts
    switch (d.type) {
      case "orders":
        try {
        out = document.getElementById('orders-list');
        out.innerHTML = d.data + '<br>';
        ts = document.getElementById('orders-date')
        ts.innerText = new Date(Date.now()).toLocaleString('en-US', { timeZone: 'America/Indiana/Indianapolis' })
        } catch (e) {
            console.log(e)
        }
        break;
      case "chats":
        out = document.getElementById('chat-playback')
        out.innerHTML = d.data + '<br>';
        ts = document.getElementById('chat-date')
        ts.innerText = new Date(Date.now()).toLocaleString('en-US', { timeZone: 'America/Indiana/Indianapolis' })
        break;
    }
  }
  return ws;
}

var loc = window.location;
var uri = 'ws:';

if (loc.protocol === 'https:') {
  uri = 'wss:';
}
uri += '//' + loc.host;
uri += '/' + 'ws';

ws = setupWs(uri);

ws.onclose = function() {
  ws = setupWs(uri);
}

var sleepSetTimeout_ctrl;

function sleep(ms) {
    clearInterval(sleepSetTimeout_ctrl);
    return new Promise(resolve => sleepSetTimeout_ctrl = setTimeout(resolve, ms));
}

async function sendOnOpen() {
  for (i = 0; i < 10; i++) {
    if (ws.readyState == WebSocket.OPEN) {
      console.log("sending good morning");
      ws.send("good morning");
      break;
    } else {
      await sleep(50);
    }
  }
}

sendOnOpen()

// Check that the websocket is healthy every 60s
setInterval(function() {
  console.log("checking for websocket connection...")
  if (!(ws.readyState == WebSocket.OPEN || ws.readyState == WebSocket.CONNECTING)) {
    ws = setupWs(uri);
  } else {
    console.log("still connected")
  }
}, 10000)
