module DashboardDragDropTests exposing (all)

import Callback
import Concourse.PipelineStatus as PipelineStatus
import Dashboard.Msgs
import Dict
import Effects
import Expect
import Html.Attributes
import Json.Encode
import Layout
import Msgs
import RemoteData
import SubPage.Msgs
import Test
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (attribute, class, style)


describe : String -> model -> List (model -> Test.Test) -> Test.Test
describe description beforeEach subTests =
    Test.describe description
        (subTests |> List.map (\f -> f beforeEach))


context : String -> (model -> b) -> List (b -> Test.Test) -> (model -> Test.Test)
context description setup subTests beforeEach =
    Test.describe description
        (subTests |> List.map (\f -> f <| setup beforeEach))


it : String -> (model -> Expect.Expectation) -> model -> Test.Test
it desc expectationFunc model =
    Test.test desc <|
        \_ -> expectationFunc model


all : Test.Test
all =
    describe "dashboard drag and drop"
        (Layout.init
            { turbulenceImgSrc = ""
            , notFoundImgSrc = ""
            , csrfToken = ""
            , authToken = ""
            , pipelineRunningKeyframes = ""
            }
            { href = ""
            , host = ""
            , hostname = ""
            , protocol = ""
            , origin = ""
            , port_ = ""
            , pathname = "/"
            , search = ""
            , hash = ""
            , username = ""
            , password = ""
            }
            |> Tuple.first
            |> Layout.handleCallback
                (Effects.SubPage 1)
                (Callback.APIDataFetched
                    (RemoteData.Success
                        ( 0
                        , { teams = [ { id = 0, name = "team" } ]
                          , pipelines =
                                [ { id = 0
                                  , name = "pipeline"
                                  , paused = False
                                  , public = True
                                  , teamName = "team"
                                  , groups = []
                                  }
                                , { id = 1
                                  , name = "other-pipeline"
                                  , paused = False
                                  , public = True
                                  , teamName = "team"
                                  , groups = []
                                  }
                                , { id = 2
                                  , name = "third-pipeline"
                                  , paused = False
                                  , public = True
                                  , teamName = "team"
                                  , groups = []
                                  }
                                ]
                          , jobs = []
                          , resources = []
                          , user =
                                Just
                                    { id = "test"
                                    , userName = "test"
                                    , name = "test"
                                    , email = "test"
                                    , teams =
                                        Dict.fromList
                                            [ ( "team", [ "owner" ] ) ]
                                    }
                          , version = "0.0.0-dev"
                          }
                        )
                    )
                )
            |> Tuple.first
        )
    <|
        let
            dragStartMsg =
                Msgs.SubMsg 1 <|
                    SubPage.Msgs.DashboardMsg <|
                        Dashboard.Msgs.DragStart "team" 0
        in
        [ context "'pipeline' card properties"
            (queryView >> pipelineCard "pipeline")
            [ it "is draggable" <|
                Query.has [ attribute <| Html.Attributes.draggable "true" ]
            , it "has dragstart listener" <|
                Event.simulate
                    (Event.custom "dragstart" <| Json.Encode.object [])
                    >> Event.expect dragStartMsg
            , it "has default margins" <|
                Query.has [ style [ ( "margin", "25px" ) ] ]
            ]
        , context "when dragging 'pipeline' card"
            (Layout.update dragStartMsg >> Tuple.first)
          <|
            let
                dragEnterMsg =
                    Msgs.SubMsg 1 <|
                        SubPage.Msgs.DashboardMsg <|
                            Dashboard.Msgs.DragOver 2

                dragEndMsg =
                    Msgs.SubMsg 1 <|
                        SubPage.Msgs.DashboardMsg <|
                            Dashboard.Msgs.DragEnd
            in
            [ it "'pipeline' card disappears" <|
                queryView
                    >> pipelineCard "pipeline"
                    >> Query.has
                        [ style
                            [ ( "width", "0" )
                            , ( "overflow", "hidden" )
                            , ( "margin", "12.5px" )
                            ]
                        ]
            , it "there are 3 drop areas" <|
                queryView
                    >> Query.findAll [ class "drop-area" ]
                    >> Query.count (Expect.equal 3)
            , it "first drop area is bigger" <|
                queryView
                    >> Query.findAll [ class "drop-area" ]
                    >> Query.first
                    >> Query.has [ style [ ( "padding", "0 198.5px" ) ] ]
            , it "there is no animation" <|
                queryView
                    >> Query.findAll [ class "drop-area" ]
                    >> Query.each
                        (Query.hasNot
                            [ style
                                [ ( "transition", "all .2s ease-in-out 0s" ) ]
                            ]
                        )
            , it "middle drop area has default size" <|
                queryView
                    >> Query.findAll [ class "drop-area" ]
                    >> Query.index 1
                    >> dropAreaNormalState
            , it "card has dragend listener" <|
                queryView
                    >> pipelineCard "pipeline"
                    >> Event.simulate
                        (Event.custom "dragend" <| Json.Encode.object [])
                    >> Event.expect dragEndMsg
            , it "rightmost drop area has dragenter listener" <|
                queryView
                    >> Query.findAll [ class "drop-area" ]
                    >> Query.index -1
                    >> Event.simulate
                        (Event.custom "dragenter" <| Json.Encode.object [])
                    >> Event.expect dragEnterMsg
            , context "after cancelling the drag"
                (Layout.update dragEndMsg >> Tuple.first)
                [ it "drop areas return to normal state" <|
                    queryView
                        >> Query.findAll [ class "drop-area" ]
                        >> Query.each dropAreaNormalState
                ]
            , context "when dragging 'pipeline' card over rightmost drop area"
                (Layout.update dragEnterMsg >> Tuple.first)
                [ it "leftmost drop area has default size" <|
                    queryView
                        >> Query.findAll [ class "drop-area" ]
                        >> Query.first
                        >> dropAreaNormalState
                , it "rightmost drop area grows, animated" <|
                    queryView
                        >> Query.findAll [ class "drop-area" ]
                        >> Query.index -1
                        >> Query.has
                            [ style
                                [ ( "padding", "0 198.5px" )
                                , ( "transition", "all .2s ease-in-out 0s" )
                                ]
                            ]
                , context "after dropping 'pipeline' card on rightmost drop area"
                    (Layout.update dragEndMsg)
                    [ it "drop areas return to normal size" <|
                        Tuple.first
                            >> queryView
                            >> Query.findAll [ class "drop-area" ]
                            >> Query.each dropAreaNormalState
                    , it "sends a request to order pipelines" <|
                        Tuple.second
                            >> Expect.equal
                                [ ( Effects.SubPage 1
                                  , Effects.SendOrderPipelinesRequest
                                        "team"
                                        [ { id = 1
                                          , name = "other-pipeline"
                                          , teamName = "team"
                                          , public = True
                                          , jobs = []
                                          , resourceError = False
                                          , status = PipelineStatus.PipelineStatusPending False
                                          }
                                        , { id = 2
                                          , name = "third-pipeline"
                                          , teamName = "team"
                                          , public = True
                                          , jobs = []
                                          , resourceError = False
                                          , status = PipelineStatus.PipelineStatusPending False
                                          }
                                        , { id = 0
                                          , name = "pipeline"
                                          , teamName = "team"
                                          , public = True
                                          , jobs = []
                                          , resourceError = False
                                          , status = PipelineStatus.PipelineStatusPending False
                                          }
                                        ]
                                        ""
                                  )
                                ]
                    , it "cards are in opposite order" <|
                        Tuple.first
                            >> queryView
                            >> Query.findAll [ class "card" ]
                            >> Expect.all
                                [ Query.count (Expect.equal 3)
                                , Query.index 0
                                    >> Query.has
                                        [ attribute <|
                                            Html.Attributes.attribute
                                                "data-pipeline-name"
                                                "other-pipeline"
                                        ]
                                , Query.index 1
                                    >> Query.has
                                        [ attribute <|
                                            Html.Attributes.attribute
                                                "data-pipeline-name"
                                                "third-pipeline"
                                        ]
                                , Query.index 2
                                    >> Query.has
                                        [ attribute <|
                                            Html.Attributes.attribute
                                                "data-pipeline-name"
                                                "pipeline"
                                        ]
                                ]
                    ]
                ]
            ]
        ]


dropAreaNormalState : Query.Single msg -> Expect.Expectation
dropAreaNormalState =
    Query.has [ style [ ( "padding", "0 50px" ) ] ]


pipelineCard : String -> Query.Single msg -> Query.Single msg
pipelineCard name =
    Query.find
        [ attribute <|
            Html.Attributes.attribute "data-pipeline-name" name
        ]


queryView : Layout.Model -> Query.Single Msgs.Msg
queryView =
    Layout.view >> Query.fromHtml
