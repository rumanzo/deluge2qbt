# deluge2qbt
deluge2qbt is cli tool for export from uTorrent\Bittorrent into qBittorrent (convert)
- [deluge2qbt](#deluge2qbt)
	- [Feature](#user-content-feature)
	- [Help](#user-content-help)
	- [Usage examples](#user-content-usage-examples)
	- [Linux](#user-content-linux)
	
Feature:
---------
 - Processing all torrents
 - Processing torrents with subdirectories and without subdirectories
 - Processing torrents with renamed files
 - Processing torrents with non-standard encodings (for example, cp1251)
 - Processing of torrents in the not ready state *
 - Save date, metrics, status. **
 - Import of tags and labels
 - Multithreading

> [!NOTE]
> \* This torrents will not be done (0%) and will need force rehash

> [!NOTE]
>\*\* The calculation of the completed parts is based only on the priority of the files in torrent

> [!NOTE]
>\*\*\* Partially downloaded torrents will be visible as 100% completed, but in fact you will need to do a rehash. Without rehash torrents not will be valid. This is due to the fact that conversion of .dat files in which parts of objects are stored is not implemented.

> [!IMPORTANT]
> Don't forget before use make backup bittorrent\utorrent, qbittorrent folder. and config %APPDATA%/Roaming/qBittorrent/qBittorrent.ini. Close all this program before.

Help:
-------

Help (from cmd or powerwhell)

```
PS C:\Users\user\Downloads> .\deluge2qbt_v1.999_amd64.exe -h
Usage:
  C:\Users\user\Downloads\deluge2qbt_v1.999_amd64.exe [OPTIONS]

Application Options:
  -s, --source=       Source directory that contains deluge files (default: C:\Users\user\AppData\Roaming\deluge\)
  -d, --destination=  Destination directory BT_backup (as default) (default: C:\Users\user\AppData\Local\qBittorrent\BT_backup\)
      --without-tags  Do not export/import tags
  -r, --replace=      Replace save paths. Important: you have to use single slashes in paths
                      Delimiter for from/to is comma - ,
                      Example: -r "D:/films,/home/user/films" -r "D:/music,/home/user/music"

  -v, --version       Show version

Help Options:
  -h, --help          Show this help message
```

Usage examples:
----------------

- If you just run application, it will processing torrents from %APPDATA%\deluge\ to %LOCALAPPDATA%\qBittorrent\BT_BACKUP\
```
C:\Users\user\Downloads> .\deluge2qbt_v999_amd64.exe
It will be performed processing from directory C:\Users\user\AppData\Roaming\deluge\ to directory C:\Users\user\AppData\Local\qBittorrent\BT_backup\
Check that the qBittorrent is turned off and the directory C:\Users\user\AppData\Local\qBittorrent\BT_backup\ and config C:\Users\user\AppData\Roaming\qBittorrent\qBittorrent.ini is backed up.

Press Enter to start

Started
1/2 Sucessfully imported 1.torrent
2/2 Sucessfully imported 2.torrent

Press Enter to exit
```

- Run application from cmd or powershell with keys, if you want change source dir or destination dir, or export/import behavior
```
C:\Users\user\Downloads> .\deluge2qbt_v999_amd64.exe -s C:\Users\user2\AppData\Roaming\deluge\
It will be performed processing from directory C:\Users\user2\AppData\Roaming\deluge\ to directory C:\Users\user\AppData\Local\qBittorrent\BT_backup\
Check that the qBittorrent is turned off and the directory C:\Users\user\AppData\Local\qBittorrent\BT_backup\ is backed up.

Press Enter to start
Started
1/3233 Sucessfully imported 1.torrent
2/3233 Sucessfully imported 2.torrent
3/3233 Sucessfully imported 3.torrent
...
3231/3233 Sucessfully imported 3231.torrent
3232/3233 Sucessfully imported 3232.torrent
3233/3233 Sucessfully imported 3233.torrent

Press Enter to exit
```

Linux or MacOs
----------
Exactly the same, just different paths and don't forget make chmod to deluge2qbt executable file
