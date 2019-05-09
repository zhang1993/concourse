module SideBar.SideBar exposing (hamburgerMenu, view)

import Concourse
import Html exposing (Html)
import Html.Attributes exposing (href, id, title)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import List.Extra
import Message.Message exposing (DomID(..), Message(..))
import Routes
import ScreenSize exposing (ScreenSize(..))
import Set exposing (Set)
import SideBar.Styles as Styles
import Views.Icon as Icon


type alias Model m =
    { m
        | expandedTeams : Set String
        , pipelines : List Concourse.Pipeline
        , hovered : Maybe DomID
        , isSideBarOpen : Bool
        , currentPipeline : Maybe Concourse.PipelineIdentifier
        , screenSize : ScreenSize.ScreenSize
    }


view : Model m -> Html Message
view model =
    if
        model.isSideBarOpen
            && not (List.isEmpty model.pipelines)
            && (model.screenSize /= ScreenSize.Mobile)
    then
        Html.div
            (id "side-bar" :: Styles.sideBar)
            (model.pipelines
                |> List.Extra.gatherEqualsBy .teamName
                |> List.map
                    (\( p, ps ) ->
                        team
                            { hovered = model.hovered
                            , isExpanded = Set.member p.teamName model.expandedTeams
                            , teamName = p.teamName
                            , pipelines = p :: ps
                            , currentPipeline = model.currentPipeline
                            }
                    )
            )

    else
        Html.text ""


team :
    { a
        | hovered : Maybe DomID
        , isExpanded : Bool
        , teamName : String
        , pipelines : List Concourse.Pipeline
        , currentPipeline : Maybe Concourse.PipelineIdentifier
    }
    -> Html Message
team ({ isExpanded, pipelines } as session) =
    Html.div
        Styles.team
        [ teamHeader session
        , if isExpanded then
            Html.div Styles.column <| List.map (pipeline session) pipelines

          else
            Html.text ""
        ]


teamHeader :
    { a
        | hovered : Maybe DomID
        , isExpanded : Bool
        , teamName : String
        , currentPipeline : Maybe Concourse.PipelineIdentifier
    }
    -> Html Message
teamHeader { hovered, isExpanded, teamName, currentPipeline } =
    let
        isHovered =
            hovered == Just (SideBarTeam teamName)

        isCurrent =
            (currentPipeline
                |> Maybe.map .teamName
            )
                == Just teamName
    in
    Html.div
        (Styles.teamHeader
            ++ [ onClick <| Click <| SideBarTeam teamName
               , onMouseEnter <| Hover <| Just <| SideBarTeam teamName
               , onMouseLeave <| Hover Nothing
               ]
        )
        [ Styles.teamIcon { isCurrent = isCurrent, isHovered = isHovered }
        , Styles.arrow
            { isHovered = isHovered
            , isExpanded = isExpanded
            }
        , Html.div
            (title teamName
                :: Styles.teamName
                    { isHovered = isHovered
                    , isCurrent = isCurrent
                    }
            )
            [ Html.text teamName ]
        ]


pipeline :
    { a
        | hovered : Maybe DomID
        , teamName : String
        , currentPipeline : Maybe Concourse.PipelineIdentifier
    }
    -> Concourse.Pipeline
    -> Html Message
pipeline { hovered, teamName, currentPipeline } p =
    let
        pipelineId =
            { pipelineName = p.name
            , teamName = teamName
            }

        isCurrent =
            currentPipeline == Just pipelineId

        isHovered =
            hovered == Just (SideBarPipeline pipelineId)
    in
    Html.div Styles.pipeline
        [ Html.div
            (Styles.pipelineIcon
                { isCurrent = isCurrent
                , isHovered = isHovered
                }
            )
            []
        , Html.a
            (Styles.pipelineLink
                { isHovered = isHovered
                , isCurrent = isCurrent
                }
                ++ [ href <|
                        Routes.toString <|
                            Routes.Pipeline { id = pipelineId, groups = [] }
                   , title p.name
                   , onMouseEnter <| Hover <| Just <| SideBarPipeline pipelineId
                   , onMouseLeave <| Hover Nothing
                   ]
            )
            [ Html.text p.name ]
        ]


hamburgerMenu :
    { a
        | screenSize : ScreenSize
        , pipelines : List Concourse.Pipeline
        , isSideBarOpen : Bool
        , hovered : Maybe DomID
    }
    -> Html Message
hamburgerMenu model =
    if model.screenSize == Mobile then
        Html.text ""

    else
        let
            isHamburgerClickable =
                not <| List.isEmpty model.pipelines
        in
        Html.div
            (id "hamburger-menu"
                :: Styles.hamburgerMenu
                    { isSideBarOpen = model.isSideBarOpen
                    , isClickable = isHamburgerClickable
                    }
                ++ [ onMouseEnter <| Hover <| Just HamburgerMenu
                   , onMouseLeave <| Hover Nothing
                   ]
                ++ (if isHamburgerClickable then
                        [ onClick <| Click HamburgerMenu ]

                    else
                        []
                   )
            )
            [ Icon.icon
                { sizePx = 54, image = "baseline-menu-24px.svg" }
              <|
                (Styles.hamburgerIcon <|
                    { isHovered =
                        isHamburgerClickable
                            && (model.hovered == Just HamburgerMenu)
                    , isActive = model.isSideBarOpen
                    }
                )
            ]
