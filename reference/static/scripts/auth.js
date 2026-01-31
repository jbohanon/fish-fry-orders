document.getElementById("submit-btn").addEventListener("click", async () => {
    const pass = document.getElementById("pswd").value
    if (pass !== "") {
        fetch("/authorize?pass="+pass).then((res) => {
            if (res.ok) {
                document.cookie = "theweakestauthintheworld="+pass
                location = "/"
            }
        })
    }
})
