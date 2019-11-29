module DragAndDropTests exposing (..)

import Application.Application as Application
import Common exposing (given, then_, when)
import Expect exposing (Expectation)
import Json.Encode as Encode
import Message.Callback as Callback
import Message.Message as Message
import Message.TopLevelMessage as TopLevelMessage exposing (TopLevelMessage)
import Test exposing (Test, describe, test)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (class)
import Url


all : Test
all =
    describe "dragging and dropping pipeline cards"
        [ test "pipeline card has dragstart listener" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedOnePipeline
                >> when iAmLookingAtThePipelineCard
                >> then_ itListensForDragStart
        , test "pipeline card should disappear when dragging starts" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedOnePipeline
                >> given iStartDraggingThePipelineCard
                >> when iAmLookingAtTheListOfPipelineCards
                >> then_ itIsEmpty
        ]


iVisitedTheDashboard _ =
    Application.init
        { turbulenceImgSrc = ""
        , notFoundImgSrc = ""
        , csrfToken = ""
        , authToken = ""
        , pipelineRunningKeyframes = ""
        }
        { protocol = Url.Http
        , host = ""
        , port_ = Nothing
        , path = "/"
        , query = Nothing
        , fragment = Nothing
        }


myBrowserFetchedOnePipeline =
    Tuple.first
        >> Application.handleCallback
            (Callback.AllPipelinesFetched <|
                Ok
                    [ { id = 0
                      , name = "pipeline"
                      , paused = False
                      , public = True
                      , teamName = "team"
                      , groups = []
                      }
                    ]
            )


iAmLookingAtThePipelineCard =
    Tuple.first
        >> Common.queryView
        >> Query.find [ class "card" ]


itListensForDragStart : Query.Single TopLevelMessage -> Expectation
itListensForDragStart =
    Event.simulate (Event.custom "dragstart" (Encode.object []))
        >> Event.expect
            (TopLevelMessage.Update <| Message.DragStart "team" 0)


iStartDraggingThePipelineCard =
    Tuple.first
        >> Application.update
            (TopLevelMessage.Update <| Message.DragStart "team" 0)


iAmLookingAtTheListOfPipelineCards =
    Tuple.first
        >> Common.queryView
        >> Query.findAll [ class "card" ]


itIsEmpty =
    Query.count (Expect.equal 0)
