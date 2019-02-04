module Dashboard.Group exposing
    ( DragState
    , Group
    , StickyHeaderConfig
    , allPipelines
    , allTeamNames
    , drop
    , group
    , groups
    , hdView
    , jobStatus
    , ordering
    , pipelineDropAreaView
    , pipelineNotSetView
    , pipelineStatus
    , shiftPipelineTo
    , shiftPipelines
    , stickyHeaderConfig
    , transition
    , view
    )

import Concourse
import Concourse.BuildStatus
import Concourse.PipelineStatus as PipelineStatus
import Dashboard.APIData exposing (APIData)
import Dashboard.Group.Tag as Tag
import Dashboard.Models as Models
import Dashboard.Msgs exposing (DragOver(..), Msg(..))
import Dashboard.Pipeline as Pipeline
import Dashboard.Styles as Styles
import Date exposing (Date)
import Html exposing (..)
import Html.Attributes exposing (..)
import Html.Events exposing (on, onMouseEnter)
import Json.Decode
import List.Extra
import Maybe.Extra
import NewTopBar.Styles as NTBS
import Ordering exposing (Ordering)
import Set
import Time exposing (Time)


type alias Group =
    { pipelines : List Models.Pipeline
    , teamName : String
    , tag : Maybe Tag.Tag
    }


ordering : Ordering Group
ordering =
    Ordering.byFieldWith Tag.ordering .tag
        |> Ordering.breakTiesWith (Ordering.byField .teamName)


type alias StickyHeaderConfig =
    { pageHeaderHeight : Float
    , pageBodyClass : String
    , sectionHeaderClass : String
    , sectionClass : String
    , sectionBodyClass : String
    }


stickyHeaderConfig : StickyHeaderConfig
stickyHeaderConfig =
    { pageHeaderHeight = NTBS.pageHeaderHeight
    , pageBodyClass = "dashboard"
    , sectionClass = "dashboard-team-group"
    , sectionHeaderClass = "dashboard-team-header"
    , sectionBodyClass = "dashboard-team-pipelines"
    }


type alias DragState =
    { teamName : String
    , from : Int
    , to : Int
    }


drop :
    DragState
    -> Group
    -> Group
drop { from, to } ({ pipelines } as g) =
    let
        draggedPipeline =
            List.Extra.getAt from pipelines

        newPipelines =
            case draggedPipeline of
                Just dragged ->
                    let
                        withoutDragged =
                            List.filter ((/=) dragged) pipelines
                    in
                    List.take to withoutDragged
                        ++ (dragged :: List.drop to withoutDragged)

                Nothing ->
                    pipelines
    in
    { g | pipelines = newPipelines }


allPipelines : APIData -> List Models.Pipeline
allPipelines data =
    data.pipelines
        |> List.map
            (\p ->
                let
                    jobs =
                        data.jobs
                            |> List.filter
                                (\j ->
                                    (j.teamName == p.teamName)
                                        && (j.pipelineName == p.name)
                                )
                in
                { id = p.id
                , name = p.name
                , teamName = p.teamName
                , public = p.public
                , jobs = jobs
                , resourceError =
                    data.resources
                        |> List.any
                            (\r ->
                                (r.teamName == p.teamName)
                                    && (r.pipelineName == p.name)
                                    && r.failingToCheck
                            )
                , status = pipelineStatus p jobs
                }
            )


pipelineStatus : Concourse.Pipeline -> List Concourse.Job -> PipelineStatus.PipelineStatus
pipelineStatus pipeline jobs =
    if pipeline.paused then
        PipelineStatus.PipelineStatusPaused

    else
        let
            isRunning =
                List.any (\job -> job.nextBuild /= Nothing) jobs

            mostImportantJobStatus =
                jobs
                    |> List.map jobStatus
                    |> List.sortWith Concourse.BuildStatus.ordering
                    |> List.head

            firstNonSuccess =
                jobs
                    |> List.filter (jobStatus >> (/=) Concourse.BuildStatusSucceeded)
                    |> List.filterMap transition
                    |> List.sort
                    |> List.head

            lastTransition =
                jobs
                    |> List.filterMap transition
                    |> List.sort
                    |> List.reverse
                    |> List.head

            transitionTime =
                case firstNonSuccess of
                    Just t ->
                        Just t

                    Nothing ->
                        lastTransition
        in
        case ( mostImportantJobStatus, transitionTime ) of
            ( _, Nothing ) ->
                PipelineStatus.PipelineStatusPending isRunning

            ( Nothing, _ ) ->
                PipelineStatus.PipelineStatusPending isRunning

            ( Just Concourse.BuildStatusPending, _ ) ->
                PipelineStatus.PipelineStatusPending isRunning

            ( Just Concourse.BuildStatusStarted, _ ) ->
                PipelineStatus.PipelineStatusPending isRunning

            ( Just Concourse.BuildStatusSucceeded, Just since ) ->
                if isRunning then
                    PipelineStatus.PipelineStatusSucceeded PipelineStatus.Running

                else
                    PipelineStatus.PipelineStatusSucceeded (PipelineStatus.Since since)

            ( Just Concourse.BuildStatusFailed, Just since ) ->
                if isRunning then
                    PipelineStatus.PipelineStatusFailed PipelineStatus.Running

                else
                    PipelineStatus.PipelineStatusFailed (PipelineStatus.Since since)

            ( Just Concourse.BuildStatusErrored, Just since ) ->
                if isRunning then
                    PipelineStatus.PipelineStatusErrored PipelineStatus.Running

                else
                    PipelineStatus.PipelineStatusErrored (PipelineStatus.Since since)

            ( Just Concourse.BuildStatusAborted, Just since ) ->
                if isRunning then
                    PipelineStatus.PipelineStatusAborted PipelineStatus.Running

                else
                    PipelineStatus.PipelineStatusAborted (PipelineStatus.Since since)


jobStatus : Concourse.Job -> Concourse.BuildStatus
jobStatus job =
    case job.finishedBuild of
        Just build ->
            build.status

        Nothing ->
            Concourse.BuildStatusPending


transition : Concourse.Job -> Maybe Time
transition job =
    case job.transitionBuild of
        Just build ->
            build.duration.finishedAt
                |> Maybe.map Date.toTime

        Nothing ->
            Nothing


shiftPipelines : Int -> Int -> Group -> Group
shiftPipelines dragIndex dropIndex group =
    if dragIndex == dropIndex then
        group

    else
        let
            pipelines =
                case
                    List.head <|
                        List.drop dragIndex <|
                            group.pipelines
                of
                    Nothing ->
                        group.pipelines

                    Just pipeline ->
                        shiftPipelineTo pipeline dropIndex group.pipelines
        in
        { group | pipelines = pipelines }



-- TODO this is pretty hard to reason about. really deeply nested and nasty. doesn't exactly relate
-- to the hd refactor as hd doesn't have the drag-and-drop feature, but it's a big contributor
-- to the 'length of this file' tire fire


shiftPipelineTo : Models.Pipeline -> Int -> List Models.Pipeline -> List Models.Pipeline
shiftPipelineTo pipeline position pipelines =
    case pipelines of
        [] ->
            if position < 0 then
                []

            else
                [ pipeline ]

        p :: ps ->
            if p.teamName /= pipeline.teamName then
                p :: shiftPipelineTo pipeline position ps

            else if p == pipeline then
                shiftPipelineTo pipeline (position - 1) ps

            else if position == 0 then
                pipeline :: p :: shiftPipelineTo pipeline (position - 1) ps

            else
                p :: shiftPipelineTo pipeline (position - 1) ps


allTeamNames : APIData -> List String
allTeamNames apiData =
    Set.union
        (Set.fromList (List.map .teamName apiData.pipelines))
        (Set.fromList (List.map .name apiData.teams))
        |> Set.toList


groups : APIData -> List Group
groups apiData =
    let
        teamNames =
            allTeamNames apiData
    in
    teamNames
        |> List.map (group (allPipelines apiData) apiData.user)


group : List Models.Pipeline -> Maybe Concourse.User -> String -> Group
group allPipelines user teamName =
    { pipelines = List.filter (.teamName >> (==) teamName) allPipelines
    , teamName = teamName
    , tag =
        case user of
            Just u ->
                Tag.tag u teamName

            Nothing ->
                Nothing
    }


view :
    { dragState : Maybe DragState
    , now : Time
    , hoveredPipeline : Maybe Models.Pipeline
    , pipelineRunningKeyframes : String
    , dragChanged : Bool
    }
    -> Group
    -> Html Msg
view { dragState, dragChanged, now, hoveredPipeline, pipelineRunningKeyframes } group =
    let
        active =
            dragState /= Nothing

        pipelines =
            if List.isEmpty group.pipelines then
                [ Pipeline.pipelineNotSetView ]

            else
                List.append
                    (List.indexedMap
                        (\idx pipeline ->
                            let
                                myPid =
                                    { teamName = pipeline.teamName
                                    , pipelineName = pipeline.name
                                    }

                                dragging =
                                    case dragState of
                                        Just { from } ->
                                            from == idx

                                        Nothing ->
                                            False
                            in
                            Html.div [ class "pipeline-wrapper" ] <|
                                (if dragging then
                                    []

                                 else
                                    [ pipelineDropAreaView
                                        { active = active
                                        , dragState = dragState
                                        , dragChanged = dragChanged
                                        }
                                        (case dragState of
                                            Just { from } ->
                                                if idx > from then
                                                    idx - 1

                                                else
                                                    idx

                                            Nothing ->
                                                idx
                                        )
                                    ]
                                )
                                    ++ [ Pipeline.pipelineView
                                            { now = now
                                            , pipeline = pipeline
                                            , hovered =
                                                hoveredPipeline
                                                    == Just pipeline
                                            , dragging = dragging
                                            , pipelineRunningKeyframes =
                                                pipelineRunningKeyframes
                                            , index = idx
                                            }
                                       ]
                        )
                        group.pipelines
                    )
                    [ pipelineDropAreaView
                        { active = active
                        , dragState = dragState
                        , dragChanged = dragChanged
                        }
                        (List.length group.pipelines - 1)
                    ]
    in
    Html.div
        [ id group.teamName
        , class "dashboard-team-group"
        , attribute "data-team-name" group.teamName
        ]
        [ Html.div
            [ style [ ( "display", "flex" ), ( "align-items", "center" ) ]
            , class stickyHeaderConfig.sectionHeaderClass
            ]
            ([ Html.div [ class "dashboard-team-name" ] [ Html.text group.teamName ]
             ]
                ++ (Maybe.Extra.maybeToList <| Maybe.map (Tag.view False) group.tag)
            )
        , Html.div [ class stickyHeaderConfig.sectionBodyClass ] pipelines
        ]


hdView : String -> Group -> Html Msg
hdView pipelineRunningKeyframes group =
    let
        header =
            [ Html.div [ class "dashboard-team-name" ] [ Html.text group.teamName ]
            ]
                ++ (Maybe.Extra.maybeToList <| Maybe.map (Tag.view True) group.tag)

        teamPipelines =
            if List.isEmpty group.pipelines then
                [ pipelineNotSetView ]

            else
                group.pipelines
                    |> List.map
                        (\p ->
                            Pipeline.hdPipelineView
                                { pipeline = p
                                , pipelineRunningKeyframes = pipelineRunningKeyframes
                                }
                        )
    in
    Html.div [ class "pipeline-wrapper" ] <|
        case teamPipelines of
            [] ->
                header

            p :: ps ->
                -- Wrap the team name and the first pipeline together so the team name is not the last element in a column
                List.append [ Html.div [ class "dashboard-team-name-wrapper" ] (header ++ [ p ]) ] ps


pipelineNotSetView : Html msg
pipelineNotSetView =
    Html.div
        [ class "card" ]
        [ Html.div
            [ style Styles.noPipelineCardHd ]
            [ Html.div
                [ style Styles.noPipelineCardTextHd ]
                [ Html.text "no pipelines set" ]
            ]
        ]


pipelineDropAreaView :
    { active : Bool, dragState : Maybe DragState, dragChanged : Bool }
    -> Int
    -> Html Msg
pipelineDropAreaView { active, dragState, dragChanged } idx =
    let
        over =
            case dragState of
                Just { to } ->
                    to == idx

                Nothing ->
                    False
    in
    Html.div
        [ classList
            [ ( "drop-area", True )
            , ( "active", active )
            ]
        , style <|
            Styles.pipelineDropArea
                { over = over, dragChanged = dragChanged }
        , on "dragenter" (Json.Decode.succeed <| DragOver idx)
        ]
        [ Html.text "" ]
