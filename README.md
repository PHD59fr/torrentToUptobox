#  TorrentToUptobox (direct link converter)

## Project Status

### ⚠️ It works ! Being refactored ... ⚠️

## Goal

➤ Why ? The main use of this application is to convert a **torrent file** to a **direct link** for several reasons:

### Pros:
- Download speed
- No heavy client for torrent download
- Availability, the file is available even if there is no more seed
- Generic protocol usable with a wget or a curl
- Your ip is not directly visible via the torrent system
- Handle bulk processing


### Cons:
- Requires an Alldebrid and Uptobox premium subscription
- Need a system to run the application
- Cannot run on a server (alldebrid restriction) but can run through a proxy


## 🧐 How it works ?
```mermaid
sequenceDiagram
TorrentToUptobox ->> TorrentToUptobox: Scan folder or only file
Note right of TorrentToUptobox:The app will scan torrent<br/>files in a folder
TorrentToUptobox->>+Alldebrid: Upload torrent file
Alldebrid-->>Torrent Host:Request the files
Note right of Alldebrid:Alldebrid downloads the<br/>files from the torrent on<br/>their servers
Torrent Host-->>Alldebrid:Download the files
Alldebrid-->>Uptobox:Upload the files
Note right of Alldebrid:Alldebrid upload the<br/>previously uploaded<br/>files to uptobox
Uptobox-->>Alldebrid:Returns a public link
Alldebrid->>TorrentToUptobox: Forward utb link
TorrentToUptobox->>Uptobox:Ask resolve public link
Note right of TorrentToUptobox:Uptobox resolve<br/>https://uptobox.com/abcdefghijk to<br/> https://www9.uptobox.com/dl/vfh-wOfezfzeh/superfile.iso
Uptobox->>TorrentToUptobox:Return a direct link
TorrentToUptobox->>TorrentToUptobox:Write the list of direct links to a file



```


## 🍰 Contributing
Contributions are what make the open source community such an amazing place to be learn, inspire, and create. Any contributions you make are **greatly appreciated**.

## ❤️ Support
A simple star to this project repo is enough to keep me motivated on this project for days. If you find your self very much excited with this project let me know with a tweet.

If you have any questions, feel free to reach out to me on [Twitter](https://twitter.com/xxPHDxx).