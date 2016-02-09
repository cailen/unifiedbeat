# Unifiedbeat

Unifiedbeat reads records from [Unified2](http://manual.snort.org/node44.html) binary files generated by network intrusion detection software and indexes the records in [Elasticsearch](https://www.elastic.co/).

Unified2 files are created by [IDS/IPS software](https://en.wikipedia.org/wiki/Intrusion_prevention_system)
such as [Snort](https://www.snort.org/) and [Suricata](http://suricata-ids.org/).

This project is the modification of a [Filebeat](https://github.com/elastic/filebeat) clone from
the November 2015 github repository.

> #### Info
>
> * [Protect the Box](https://medium.com/@cleesmith/protect-the-box-c245acbaae81#.59j14oijl)

***

### January 17, 2016

#### Usage

1. download a linux 64bit [release](https://github.com/cleesmith/unifiedbeat/releases)
1. or, if you wish to build from source:
  * ```git clone https://github.com/cleesmith/unifiedbeat```
  * ```cd unifiedbeat```
  * godep usage:
    * ```export GO15VENDOREXPERIMENT=1```
    * ```godep save```
  * ```go build```
    * if building on linux 64bit platform, or if building on mac/windows do:
      * ```env GOOS=linux GOARCH=amd64 go build```
  * copy/scp unifiedbeat binary file to the server with unified2 files to be indexed
1. ```mkdir unifiedbeat```
1. ```cd unifiedbeat```
1. copy or scp the unifiedbeat binary file to the unifiedbeat folder
1. ```curl -XPUT 'http://localhost:9200/_template/unifiedbeat' -d@etc/unifiedbeat.template.json```
1. ```rm .unifiedbeat``` if exists ... this file tracks the previous positions within the unified2 files being tailed and indexed
1. ```nano or vim etc/unifiedbeat.yml``` then change:
  * unifiedbeat:
    * rules:
      * gen_msg_map_path: **?**  _# the absolute full path, typically: /etc/snort/gen-msg.map_
      * paths: **?**  _# where are the .rules files, typically: /etc/snort/rules/*.rules_
    * prospectors:
      * paths: **?**  _# where are the unfied2 files, typically: /var/log/snort/snort.log*_
  * . &nbsp; . &nbsp; .
  * output:
    * elasticsearch:
      * hosts: ["**?.?.?.?:9200**"]  _# elasticsearch's ip:port - most securely/typically on the same host as Snort_
1. ```cp etc/unifiedbeat.yml /etc/unifiedbeat.yml``` ... this is not required but typically done
1. **./unifiedbeat** -c /etc/unifiedbeat.yml
  * typically this command would be in a systemd, Upstart, or SysV (init.d) script
  * for a quick test use: ```nohup ./unifiedbeat -c /etc/unifiedbeat.yml &```
1. now, use Kibana to see what's up with your server and network

***

#### Overview

![Overview](https://raw.githubusercontent.com/cleesmith/unifiedbeat/master/screenshots/unifiedbeat.png "overview of unifiedbeat processing")

***

### Kibana screenshots

![Dashboard](https://raw.githubusercontent.com/cleesmith/unifiedbeat/master/screenshots/kibana_dashboard.png "example Kibana dashboard")

> this is just a simple example of a Kibana dashboard and not very useful for security analysts

> see kibana/export.json to import the provided dashboard, search, and visualizations into Kibana

> new to Kibana? this YouTube [playlist](https://www.youtube.com/playlist?list=PLhLSfisesZIvA8ad1J2DSdLWnTPtzWSfI) is helpful

***

#### Event record as shown in Kibana's Discover

![Event](https://raw.githubusercontent.com/cleesmith/unifiedbeat/master/screenshots/kibana_event_record.png "Kibana Discover event record")

> notice the **signature** and **rule_raw** fields

***

#### Packet record as shown in Kibana's Discover

![Packet](https://raw.githubusercontent.com/cleesmith/unifiedbeat/master/screenshots/kibana_packet_record.png "Kibana Discover packet record")

> notice the human readable **packet_dump** field with all layers shown in both hex and text

***

### Sense screenshots

#### Event type document in ElasticSearch

![Event](https://raw.githubusercontent.com/cleesmith/unifiedbeat/master/screenshots/unifiedbeat_event.png "typical Event type document in ElasticSearch")

***

#### Packet type document in ElasticSearch

![Packet](https://raw.githubusercontent.com/cleesmith/unifiedbeat/master/screenshots/unifiedbeat_packet.png "typical Packet type document in ElasticSearch")

***

### November 29, 2015

Initial git clone of [Filebeat](https://github.com/elastic/filebeat) to alter into Unifiedbeat.

> Most changes are only required in the ```input/file.go``` and ```harvester/log.go``` files to
> alter Filebeat into Unifiedbeat.

***
***
