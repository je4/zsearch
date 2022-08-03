package mediaserver

type Mediaserver interface {
	IsMediaserverURL(url string) (string, string, bool)
	GetCollectionByName(name string) (*Collection, error)
	GetCollectionById(id int64) (*Collection, error)
	CreateMasterUrl(collection, signature, url string, public bool) error
	GetMetadata(collection, signature string) (*Metadata, error)
	GetFulltext(collection, signature string) (string, error)
	FindByUrn(urn string) (string, string, error)
	GetOriginalUrn(collection, signature string) (string, error)
	GetUrl(collection, signature, function string) (string, error)
}
