package blueclaw

import "os"

type CompanyHostDirectory struct {
	Path string
	Mode os.FileMode
}

func CompanyHostStateDirectories() []CompanyHostDirectory {
	return []CompanyHostDirectory{
		{Path: CompanyHostStateRoot, Mode: CompanyHostStateRootMode},
		{Path: CompanyHostCompaniesRoot, Mode: CompanyHostStateRootMode},
		{Path: CompanyHostMediaRootPath, Mode: 0o750},
		{Path: CompanyHostMediaBucketPath, Mode: 0o750},
	}
}
