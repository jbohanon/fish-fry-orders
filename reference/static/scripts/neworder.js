document.getElementById("submit-btn").addEventListener("click", async (evt) => {
    let friedQ = document.querySelector('input[name="fried"]:checked')
    let fried = friedQ === null ? 0 : friedQ.value
    if (fried === "-1") {
        fried = document.getElementById("fried-other-text").value
    }
    let bakedQ = document.querySelector('input[name="baked"]:checked')
    let baked = bakedQ === null ? 0 : bakedQ.value
    if (baked === "-1") {
        baked = document.getElementById("baked-other-text").value
    }
    let kidsQ = document.querySelector('input[name="kids"]:checked')
    let kids = kidsQ === null ? 0 : kidsQ.value
    if (kids === "-1") {
        kids = document.getElementById("kids-other-text").value
    }

    let ordId = document.getElementById("order-id").innerText
    let vehicle = document.getElementById("vehicle").value

    const resp = await fetch("/submit", {
        method: "POST",
        headers: {
            "Content-Type": "application/json",
        },
        body: JSON.stringify({
            id: Number(ordId),
            fried: Number(fried),
            baked: Number(baked),
            kids: Number(kids),
            vehicle: vehicle.slice(0, 40),
        }),
    })
    console.log(resp)
    location.reload()
})
