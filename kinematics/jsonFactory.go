package kinematics

import (
	"encoding/json"
	"io/ioutil"
	"math"

	"github.com/go-errors/errors"

	"go.viam.com/core/config"
	"go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"
)

// AutoGenerated TODO
type AutoGenerated struct {
	Model struct {
		Manufacturer string `json:"manufacturer"`
		Name         string `json:"name"`
		Links        []struct {
			ID             string             `json:"id"`
			Parent         string             `json:"parent"`
			Translation    config.Translation `json:"translation"`
			Orientation    config.Orientation `json:"orientation"`
			SetOrientation bool               `json:"setorientation"`
		} `json:"links"`
		Joints []struct {
			ID     string `json:"id"`
			Type   string `json:"type"`
			Parent string `json:"parent"`
			Axis   struct {
				X float64 `json:"x"`
				Y float64 `json:"y"`
				Z float64 `json:"z"`
			} `json:"axis"`
			Max       float64 `json:"max"`
			Min       float64 `json:"min"`
			Direction string  `json:"direction"`
		} `json:"joints"`
		// Home position of joints. Optional, if not provided will be set to all 0.
		Home       []float64       `json:"home"`
		Tolerances *DistanceConfig `json:"tolerances"`
	} `json:"model"`
}

// ParseJSONFile TODO
func ParseJSONFile(filename string) (*Model, error) {
	jsonData, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.Errorf("failed to read json file: %w", err)
	}
	return ParseJSON(jsonData)
}

// ParseJSON TODO
func ParseJSON(jsonData []byte) (*Model, error) {
	model := NewModel()
	id2frame := make(map[string]*Frame)
	m := AutoGenerated{}

	err := json.Unmarshal(jsonData, &m)
	if err != nil {
		return nil, errors.Errorf("failed to unmarshall json file %w", err)
	}

	model.manufacturer = m.Model.Manufacturer
	model.name = m.Model.Name

	// Create world frame
	wFrame := NewFrame()
	wFrame.IsWorld = true
	model.Add(wFrame)
	id2frame["world"] = wFrame

	for _, link := range m.Model.Links {
		if link.ID == "world" {
			return model, errors.New("reserved word: cannot name a link 'world'")
		}
		frame := NewFrame()
		model.Add(frame)
		id2frame[link.ID] = frame
		frame.Name = link.ID
	}
	for _, joint := range m.Model.Joints {
		if joint.ID == "world" {
			return model, errors.New("reserved word: cannot name a joint 'world'")
		}
		frame := NewFrame()
		model.Add(frame)
		id2frame[joint.ID] = frame
		frame.Name = joint.ID
	}

	for _, link := range m.Model.Links {
		frameA := id2frame[link.Parent]
		frameB := id2frame[link.ID]

		linkT := NewTransform()
		linkT.SetName(link.ID)

		linkT.SetEdgeDescriptor(model.AddEdge(frameA, frameB))
		model.Edges[linkT.GetEdgeDescriptor()] = linkT
		linkT.t = spatialmath.NewDualQuaternion()

		if link.SetOrientation {
			newOV := &spatialmath.OrientationVec{utils.DegToRad(link.Orientation.TH), link.Orientation.X, link.Orientation.Y, link.Orientation.Z}
			linkT.t = spatialmath.NewDualQuaternionFromRotation(newOV)
		}

		linkT.t.SetTranslation(link.Translation.X, link.Translation.Y, link.Translation.Z)
	}

	// Now we add all of the transforms. Will eventually support: "cylindrical|fixed|helical|prismatic|revolute|spherical"
	for _, joint := range m.Model.Joints {

		// TODO(pl): Make this a switch once we support more than one joint type
		if joint.Type == "revolute" {
			// TODO(pl): Add speed, wraparound, etc
			frameA := id2frame[joint.Parent]
			frameB := id2frame[joint.ID]
			dir := joint.Direction
			// Check for valid joint direction
			if dir != "" && dir != "cw" && dir != "ccw" {
				return nil, errors.Errorf("unsupported joint direction: %s", dir)
			}

			rev := NewJoint(1, 1, dir)
			rev.SetEdgeDescriptor(model.AddEdge(frameA, frameB))
			model.Edges[rev.GetEdgeDescriptor()] = rev

			rev.max = append(rev.max, joint.Max*math.Pi/180)
			rev.min = append(rev.min, joint.Min*math.Pi/180)

			// TODO(pl): Add default on z
			// TODO(pl): Enforce between 0 and 1
			rev.SpatialMat.Set(0, 0, joint.Axis.X)
			rev.SpatialMat.Set(1, 0, joint.Axis.Y)
			rev.SpatialMat.Set(2, 0, joint.Axis.Z)
			rev.SetAxesFromSpatial()

			rev.SetName(joint.ID)
		} else {
			return nil, errors.Errorf("unsupported joint type detected: %v", joint.Type)
		}
	}

	model.Update()
	if m.Model.Home != nil {
		model.Home = m.Model.Home
	} else {
		for i := 0; i < len(model.Joints); i++ {
			model.Home = append(model.Home, 0)
		}
	}

	if m.Model.Tolerances != nil {
		model.DistCfg = *m.Model.Tolerances
	}

	return model, err
}
