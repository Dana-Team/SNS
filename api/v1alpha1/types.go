package v1alpha1

type Phase string

//Sns phases
const (
	Missing Phase = "missing"
	Created Phase = "created"
)

//MetaGroup
const MetaGroup = "dana.hns.io/"

//Labels
const (
	Parent = MetaGroup + "parent"
	Hns    = MetaGroup + "subnamespace"
)

//Annotations
const (
	Role    = MetaGroup + "role"
	Depth   = MetaGroup + "depth"
	Pointer = MetaGroup + "pointer"
)

//Finalizers
const (
	NsFinalizer = MetaGroup + "delete-sns"
)
