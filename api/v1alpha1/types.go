package v1alpha1

type Phase string

//Sns phases
const (
	Missing Phase = "Missing"
	Created Phase = "Created"
	None    Phase = ""
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
	Role       = MetaGroup + "role"
	Depth      = MetaGroup + "depth"
	Pointer    = MetaGroup + "pointer"
	SnsPointer = MetaGroup + "sns-pointer"
)

//Finalizers
const (
	NsFinalizer = MetaGroup + "delete-sns"
)
