# humbletwitter
A Twitter client for Macintosh Classic.

Requires a server that runs Go and Netatalk v2. The server can be a Raspberry Pi.
You also have to build a `libatalk.a` out of the Netatalk source, which can be tricky and when I figure out exact steps for how to do that again I'll let you know. (If you can `make` netatalk v2 from source with DDP support, you're most of the way.)

Also, like, requires a Macintosh with HyperCard, some way of loading the included HyperCard stack (in the disk image) into your Macintosh, and some way of having it talk to your server. 
This program uses AppleTalk (DDP) since it can sometimes be easier to get an adapter or router betweeen LocalTalk and Ethernet that transfers AppleTalk packets, than one that transfers IP packets. For example, the AsantéTalk router I have only does AppleTalk. But if you have an IP router, just write a regular Go server and your own HyperCard stack.

If you want to make HyperCard stacks that do networking or other cool things, it pays to have a copies of some of Apple's obsolete XCMDs. Take a look at http://www.staticky.com/mirrors/ftp.apple.com/developer/Tool_Chest/Development_Platforms/HyperCard_Related/APDA_HyperCard_Toolkits/ (and definitely check out the rest of the old Apple FTP site mirrored there.)

The HumbleMacintosh Twitter stack uses the ATP and NBP XCMDs from the [AppleTalk HyperCard Toolkit](http://www.staticky.com/mirrors/ftp.apple.com/developer/Tool_Chest/Development_Platforms/HyperCard_Related/APDA_HyperCard_Toolkits/AppleTalk_ToolKit.sit.hqx), in particular the HyperStation example stack by Donald Koscheka (1987). 
