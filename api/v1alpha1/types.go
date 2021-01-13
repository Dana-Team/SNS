package v1alpha1

type Phase string

//Sns phases
const (
	Missing Phase = "Missing"
	Created Phase = "Created"
	None    Phase = ""
)

const (
	Root   string = "root"
	NoRole string = "none"
	Leaf   string = "leaf"
)

//MetaGroup
const MetaGroup = "dana.hns.io/"

//Labels
const (
	Hns    = MetaGroup + "subnamespace"
	Parent = MetaGroup + "parent"
)

//Annotations
const (
	Role       = MetaGroup + "role"
	Depth      = MetaGroup + "depth"
	Pointer    = MetaGroup + "pointer"
	SnsPointer = MetaGroup + "sns-pointer"
)

//Finalizers
const (
	NsFinalizer = MetaGroup + "delete-sns"
)
