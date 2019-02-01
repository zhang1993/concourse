module Dashboard.SubState exposing (SubState, tick)

import Dashboard.Group as Group
import Time exposing (Time)


type alias SubState =
    { now : Time
    , dragState : Group.DragState
    }


tick : Time.Time -> SubState -> SubState
tick now substate =
    { substate | now = now }
