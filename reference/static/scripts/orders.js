async function orderPulled(ordId) {
    await fetch("/pulled", {
        body: JSON.stringify({Id: Number(ordId)}),
        method: "POST",
    })
}
async function orderComplete(ordId) {
    await fetch("/complete", {
        body: JSON.stringify({Id: Number(ordId)}),
        method: "POST",
    })
}
