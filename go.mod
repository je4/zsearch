module github.com/je4/zsearch/v2

go 1.18

replace github.com/je4/zsearch/v2 => ./

replace github.com/je4/salon-digital/v2 => ../salon-digital/

//replace github.com/je4/zsync => ../zsync
//replace github.com/je4/FairService/v2 => ../FairService/
//replace github.com/je4/utils/v2 => ../utils/

require (
	github.com/BurntSushi/toml v1.0.0
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/araddon/dateparse v0.0.0-20210429162001-6b43995a97de
	github.com/blevesearch/bleve/v2 v2.3.2
	github.com/bluele/gcache v0.0.2
	github.com/channelmeter/iso8601duration v0.0.0-20150204201828-8da3af7a2a61
	github.com/dgraph-io/badger/v3 v3.2103.2
	github.com/drgrib/maps v0.0.0-20190206181841-390503673595
	github.com/go-resty/resty/v2 v2.6.0
	github.com/go-sql-driver/mysql v1.6.0
	github.com/golang/snappy v0.0.4
	github.com/goph/emperror v0.17.2
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/securecookie v1.1.1
	github.com/gorilla/sessions v1.2.1
	github.com/htfy96/reformism v0.0.0-20160819020323-e5bfca398e73
	github.com/je4/FairService/v2 v2.0.6
	github.com/je4/salon-digital/v2 v2.0.0-00010101000000-000000000000
	github.com/je4/sitemap v1.0.1-0.20210914120028-a4ef87562716
	github.com/je4/utils/v2 v2.0.6
	github.com/je4/zsync v0.0.0-20211108172845-6b701afd5ef8
	github.com/juliangruber/go-intersect v1.0.0
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/opensearch-project/opensearch-go v1.0.0
	github.com/pkg/errors v0.9.1
	github.com/vanng822/go-solr v0.10.0
	go.mongodb.org/mongo-driver v1.7.3
	golang.org/x/image v0.0.0-20191009234506-e7c1f5e7dbb8
	golang.org/x/net v0.0.0-20211101193420-4a448f8816b3
	google.golang.org/api v0.60.0
)

require (
	cloud.google.com/go v0.97.0 // indirect
	emperror.dev/errors v0.8.0 // indirect
	github.com/RoaringBitmap/roaring v0.9.4 // indirect
	github.com/acomagu/bufpipe v1.0.3 // indirect
	github.com/bits-and-blooms/bitset v1.2.0 // indirect
	github.com/blend/go-sdk v1.20211025.3 // indirect
	github.com/blevesearch/bleve_index_api v1.0.1 // indirect
	github.com/blevesearch/go-porterstemmer v1.0.3 // indirect
	github.com/blevesearch/gtreap v0.1.1 // indirect
	github.com/blevesearch/mmap-go v1.0.3 // indirect
	github.com/blevesearch/scorch_segment_api/v2 v2.1.0 // indirect
	github.com/blevesearch/segment v0.9.0 // indirect
	github.com/blevesearch/snowballstem v0.9.0 // indirect
	github.com/blevesearch/upsidedown_store_api v1.0.1 // indirect
	github.com/blevesearch/vellum v1.0.7 // indirect
	github.com/blevesearch/zapx/v11 v11.3.3 // indirect
	github.com/blevesearch/zapx/v12 v12.3.3 // indirect
	github.com/blevesearch/zapx/v13 v13.3.3 // indirect
	github.com/blevesearch/zapx/v14 v14.3.3 // indirect
	github.com/blevesearch/zapx/v15 v15.3.3 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/dgraph-io/ristretto v0.1.0 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/emirpasic/gods v1.12.0 // indirect
	github.com/felixge/httpsnoop v1.0.2 // indirect
	github.com/go-git/gcfg v1.5.0 // indirect
	github.com/go-git/go-billy/v5 v5.3.1 // indirect
	github.com/go-git/go-git/v5 v5.4.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/googleapis/gax-go/v2 v2.1.1 // indirect
	github.com/gosimple/unidecode v1.0.1 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/je4/HandleCreator/v2 v2.0.3 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kevinburke/ssh_config v1.1.0 // indirect
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/lib/pq v1.10.3 // indirect
	github.com/machinebox/progress v0.2.0 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/minio/sha256-simd v1.0.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mschoch/smat v0.2.0 // indirect
	github.com/pkg/sftp v1.13.4 // indirect
	github.com/rs/xid v1.3.0 // indirect
	github.com/sergi/go-diff v1.2.0 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/snabb/diagio v1.0.0 // indirect
	github.com/xanzy/ssh-agent v0.3.1 // indirect
	go.etcd.io/bbolt v1.3.5 // indirect
	go.opencensus.io v0.23.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519 // indirect
	golang.org/x/sys v0.0.0-20211102192858-4dd72447c267 // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/resty.v1 v1.12.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
)

require (
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Microsoft/go-winio v0.5.1 // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20210920160938-87db9fbc61c7 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/golang/glog v1.0.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/flatbuffers v2.0.0+incompatible // indirect
	github.com/gosimple/slug v1.12.0
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/minio/minio-go/v7 v7.0.15 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	golang.org/x/oauth2 v0.0.0-20211028175245-ba495a64dcb5 // indirect
	google.golang.org/genproto v0.0.0-20211102202547-e9cf271f7f2c // indirect
	google.golang.org/grpc v1.42.0 // indirect
	gopkg.in/ini.v1 v1.63.2 // indirect
)
