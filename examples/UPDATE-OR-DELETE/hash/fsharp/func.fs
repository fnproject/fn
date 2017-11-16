open System
open System.IO
open System.Security.Cryptography

let createChecksum (data:byte array) =
    use md5 = MD5.Create ()
    let hash = data |> md5.ComputeHash
    BitConverter.ToString(hash).Replace("-", "").ToLower ()

let downloadRemoteImageFile uri =
    async {
        let req = System.Net.HttpWebRequest.Create (System.Uri (uri)) :?> System.Net.HttpWebRequest
        let! res = req.AsyncGetResponse ()
        let stm = res.GetResponseStream ()
        use ms = new MemoryStream ()
        stm.CopyTo(ms)
        return ms.ToArray ()
    }

[<EntryPoint>]
let main argv =
    let isPipedInput =
        try
            Console.KeyAvailable |> ignore
            false
        with
        | _ -> true

    match isPipedInput with
    | true ->
        let input = stdin
        async {
            let! uri = input.ReadToEndAsync () |> Async.AwaitTask
            return! downloadRemoteImageFile uri
        } |> Async.RunSynchronously
        |> createChecksum
        |> printfn "%s"
    | false -> () // if nothing is being piped in, then exit
    0