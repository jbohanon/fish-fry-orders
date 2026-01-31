async function submitChat(from) {
    let chatText = document.getElementById("chat-text")
    const resp = await fetch("/newChat", {
        method: "POST",
        body: JSON.stringify({
            from: from,
            data: chatText.value,
        }),
    })
    if (!resp.ok) {
        console.log("chat submit failed")
        alert("error occurred submitting chat")
    }
    chatText.value = ""
}
