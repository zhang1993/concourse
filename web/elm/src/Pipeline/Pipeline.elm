module Pipeline.Pipeline exposing
    ( Flags
    , Model
    , changeToPipelineAndGroups
    , documentTitle
    , getUpdateMessage
    , handleCallback
    , handleDelivery
    , init
    , subscriptions
    , update
    , view
    )

import Colors
import Concourse
import Concourse.Cli as Cli
import Dict
import EffectTransformer exposing (ET)
import Html exposing (Html)
import Html.Attributes
    exposing
        ( class
        , download
        , href
        , id
        , src
        , style
        )
import Html.Attributes.Aria exposing (ariaLabel)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Http
import Json.Decode
import Json.Encode
import Keyboard
import Login.Login as Login
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Message exposing (Hoverable(..), Message(..))
import Message.Subscription exposing (Delivery(..), Interval(..), Subscription(..))
import Message.TopLevelMessage exposing (TopLevelMessage(..))
import Pipeline.Styles as Styles
import RemoteData exposing (WebData)
import Routes
import ScreenSize exposing (ScreenSize(..))
import StrictEvents exposing (onLeftClickOrShiftLeftClick)
import Svg
import Svg.Attributes as SvgAttributes
import UpdateMsg exposing (UpdateMsg)
import UserState exposing (UserState)
import Views.PauseToggle as PauseToggle
import Views.Styles
import Views.TopBar as TopBar
import Views.Views as Views


type alias Model =
    Login.Model
        { pipelineLocator : Concourse.PipelineIdentifier
        , pipeline : WebData Concourse.Pipeline
        , fetchedJobs : Maybe Json.Encode.Value
        , fetchedResources : Maybe Json.Encode.Value
        , renderedJobs : Maybe Json.Encode.Value
        , renderedResources : Maybe Json.Encode.Value
        , concourseVersion : String
        , turbulenceImgSrc : String
        , experiencingTurbulence : Bool
        , selectedGroups : List String
        , hideLegend : Bool
        , hideLegendCounter : Float
        , isToggleLoading : Bool
        , hovered : Maybe Hoverable
        , screenSize : ScreenSize
        }


type alias Flags =
    { pipelineLocator : Concourse.PipelineIdentifier
    , turbulenceImgSrc : String
    , selectedGroups : List String
    }


init : Flags -> ( Model, List Effect )
init flags =
    let
        model =
            { concourseVersion = ""
            , turbulenceImgSrc = flags.turbulenceImgSrc
            , pipelineLocator = flags.pipelineLocator
            , pipeline = RemoteData.NotAsked
            , fetchedJobs = Nothing
            , fetchedResources = Nothing
            , renderedJobs = Nothing
            , renderedResources = Nothing
            , experiencingTurbulence = False
            , hideLegend = False
            , hideLegendCounter = 0
            , isToggleLoading = False
            , selectedGroups = flags.selectedGroups
            , isUserMenuExpanded = False
            , hovered = Nothing
            , screenSize = Desktop
            }
    in
    ( model
    , [ FetchPipeline flags.pipelineLocator
      , FetchVersion
      , ResetPipelineFocus
      , GetScreenSize
      ]
    )


changeToPipelineAndGroups :
    { pipelineLocator : Concourse.PipelineIdentifier
    , selectedGroups : List String
    }
    -> ET Model
changeToPipelineAndGroups { pipelineLocator, selectedGroups } ( model, effects ) =
    if model.pipelineLocator == pipelineLocator then
        let
            ( newModel, newEffects ) =
                renderIfNeeded ( { model | selectedGroups = selectedGroups }, [] )
        in
        ( newModel, effects ++ newEffects ++ [ ResetPipelineFocus ] )

    else
        let
            ( newModel, newEffects ) =
                init
                    { pipelineLocator = pipelineLocator
                    , selectedGroups = selectedGroups
                    , turbulenceImgSrc = model.turbulenceImgSrc
                    }
        in
        ( newModel, effects ++ newEffects )


timeUntilHidden : Float
timeUntilHidden =
    10 * 1000


timeUntilHiddenCheckInterval : Float
timeUntilHiddenCheckInterval =
    1 * 1000


getUpdateMessage : Model -> UpdateMsg
getUpdateMessage model =
    case model.pipeline of
        RemoteData.Failure _ ->
            UpdateMsg.NotFound

        _ ->
            UpdateMsg.AOK


handleCallback : Callback -> ET Model
handleCallback callback ( model, effects ) =
    let
        redirectToLoginIfUnauthenticated status =
            if status.code == 401 then
                [ RedirectToLogin ]

            else
                []
    in
    case callback of
        PipelineFetched (Ok pipeline) ->
            ( { model | pipeline = RemoteData.Success pipeline }
            , effects
                ++ [ FetchJobs model.pipelineLocator
                   , FetchResources model.pipelineLocator
                   ]
            )

        PipelineFetched (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 404 then
                        ( { model | pipeline = RemoteData.Failure err }
                        , effects
                        )

                    else
                        ( model
                        , effects ++ redirectToLoginIfUnauthenticated status
                        )

                _ ->
                    renderIfNeeded
                        ( { model | experiencingTurbulence = True }
                        , effects
                        )

        PipelineToggled _ (Ok ()) ->
            ( { model
                | pipeline =
                    RemoteData.map
                        (\p -> { p | paused = not p.paused })
                        model.pipeline
                , isToggleLoading = False
              }
            , effects
            )

        PipelineToggled _ (Err err) ->
            let
                newModel =
                    { model | isToggleLoading = False }
            in
            case err of
                Http.BadStatus { status } ->
                    ( newModel
                    , effects ++ redirectToLoginIfUnauthenticated status
                    )

                _ ->
                    ( newModel, effects )

        JobsFetched (Ok fetchedJobs) ->
            renderIfNeeded
                ( { model
                    | fetchedJobs = Just fetchedJobs
                    , experiencingTurbulence = False
                  }
                , effects
                )

        JobsFetched (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    ( model, effects ++ redirectToLoginIfUnauthenticated status )

                _ ->
                    renderIfNeeded
                        ( { model
                            | fetchedJobs = Nothing
                            , experiencingTurbulence = True
                          }
                        , effects
                        )

        ResourcesFetched (Ok fetchedResources) ->
            renderIfNeeded
                ( { model
                    | fetchedResources = Just fetchedResources
                    , experiencingTurbulence = False
                  }
                , effects
                )

        ResourcesFetched (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    ( model, effects ++ redirectToLoginIfUnauthenticated status )

                _ ->
                    renderIfNeeded
                        ( { model
                            | fetchedResources = Nothing
                            , experiencingTurbulence = True
                          }
                        , effects
                        )

        VersionFetched (Ok version) ->
            ( { model
                | concourseVersion = version
                , experiencingTurbulence = False
              }
            , effects
            )

        VersionFetched (Err _) ->
            ( { model | experiencingTurbulence = True }, effects )

        ScreenResized { viewport } ->
            ( { model | screenSize = ScreenSize.fromWindowSize viewport.width }
            , effects
            )

        _ ->
            ( model, effects )


handleDelivery : Delivery -> ET Model
handleDelivery delivery ( model, effects ) =
    case delivery of
        KeyDown keyEvent ->
            ( { model | hideLegend = False, hideLegendCounter = 0 }
            , if keyEvent.code == Keyboard.F then
                effects ++ [ ResetPipelineFocus ]

              else
                effects
            )

        Moused ->
            ( { model | hideLegend = False, hideLegendCounter = 0 }, effects )

        ClockTicked OneSecond _ ->
            if model.hideLegendCounter + timeUntilHiddenCheckInterval > timeUntilHidden then
                ( { model | hideLegend = True }, effects )

            else
                ( { model | hideLegendCounter = model.hideLegendCounter + timeUntilHiddenCheckInterval }
                , effects
                )

        ClockTicked FiveSeconds _ ->
            ( model, effects ++ [ FetchPipeline model.pipelineLocator ] )

        ClockTicked OneMinute _ ->
            ( model, effects ++ [ FetchVersion ] )

        WindowResized width _ ->
            ( { model | screenSize = ScreenSize.fromWindowSize width }, effects )

        _ ->
            ( model, effects )


update : Message -> ET Model
update msg ( model, effects ) =
    case msg of
        ToggleGroup group ->
            ( model
            , effects
                ++ [ NavigateTo <|
                        getNextUrl
                            (toggleGroup group model.selectedGroups model.pipeline)
                            model
                   ]
            )

        SetGroups groups ->
            ( model, effects ++ [ NavigateTo <| getNextUrl groups model ] )

        TogglePipelinePaused pipelineIdentifier paused ->
            ( { model | isToggleLoading = True }
            , effects
                ++ [ SendTogglePipelineRequest
                        pipelineIdentifier
                        paused
                   ]
            )

        Hover hoverable ->
            ( { model | hovered = hoverable }, effects )

        _ ->
            ( model, effects )


getPinnedResources : Model -> List ( String, Concourse.Version )
getPinnedResources model =
    case model.fetchedResources of
        Nothing ->
            []

        Just res ->
            Json.Decode.decodeValue (Json.Decode.list Concourse.decodeResource) res
                |> Result.withDefault []
                |> List.filterMap (\r -> Maybe.map (\v -> ( r.name, v )) r.pinnedVersion)


subscriptions : List Subscription
subscriptions =
    [ OnClockTick OneMinute
    , OnClockTick FiveSeconds
    , OnClockTick OneSecond
    , OnMouse
    , OnKeyDown
    , OnWindowResize
    ]


documentTitle : Model -> String
documentTitle model =
    model.pipelineLocator.pipelineName


view : UserState -> Model -> Html Message
view userState model =
    myView userState model |> Views.toHtml


myView : UserState -> Model -> Views.View Message
myView userState model =
    Views.div
        Views.Unidentified
        [ Views.style "height" "100%" ]
        []
        [ Views.div
            (Views.Id "page-including-top-bar")
            Views.Styles.pipelinePageIncludingTopBar
            []
            [ if model.screenSize == Phone then
                topBarPhone userState model

              else
                topBarOther userState model
            , Views.div
                (Views.Id "page-below-top-bar")
                [ Views.style "height" "100%"
                , Views.style "box-sizing" "border-box"
                , Views.style "padding-top" <|
                    case model.screenSize of
                        Phone ->
                            "112px"

                        _ ->
                            "54px"
                ]
                []
                [ viewSubPage model ]
            ]
        ]


topBarPhone : UserState -> Model -> Views.View Message
topBarPhone userState model =
    Views.div
        (Views.Id "top-bar-app")
        ((Views.Styles.pipelineTopBar <| isPaused model.pipeline)
            ++ [ Views.style "flex-direction" "column" ]
        )
        []
        [ Views.div
            Views.Unidentified
            [ Views.style "display" "flex"
            , Views.style "justify-content" "space-between"
            , Views.style "border-bottom" <|
                "1px solid "
                    ++ (if isPaused model.pipeline then
                            Colors.pausedTopbarSeparator

                        else
                            Colors.background
                       )
            ]
            []
            [ Views.a Views.Unidentified
                [ Views.style "background-image" "url(/public/images/concourse-logo-white.svg)"
                , Views.style "background-position" "50% 50%"
                , Views.style "background-repeat" "no-repeat"
                , Views.style "background-size" "42px 42px"
                , Views.style "width" "54px"
                , Views.style "height" "54px"
                ]
                [ href "/" ]
                []
            , Views.div
                (Views.Id "login-component")
                [ Views.style "max-width" "100%" ]
                []
                (Login.myViewLoginState
                    userState
                    model.isUserMenuExpanded
                    (isPaused model.pipeline)
                )
            ]
        , Views.div
            Views.Unidentified
            [ Views.style "display" "flex"
            , Views.style "justify-content" "space-between"
            ]
            []
            [ Views.div
                Views.Unidentified
                [ Views.style "font-size" "18px"
                , Views.style "padding" "15px"
                , Views.style "text-overflow" "ellipsis"
                , Views.style "overflow" "hidden"
                , Views.style "white-space" "nowrap"
                ]
                []
                [ Views.text
                    (model.pipeline
                        |> RemoteData.map .name
                        |> RemoteData.withDefault
                            model.pipelineLocator.pipelineName
                    )
                ]
            , Views.div
                Views.Unidentified
                [ Views.style "display" "flex"
                , Views.style "border-left" <|
                    "1px solid "
                        ++ (if isPaused model.pipeline then
                                Colors.pausedTopbarSeparator

                            else
                                Colors.background
                           )
                ]
                []
                [ Views.div
                    (Views.Id "pin-icon")
                    ([ Views.style "position" "relative"
                     , Views.style "margin" "8.5px"
                     ]
                        ++ (if model.hovered == Just PinIcon then
                                [ Views.style "background-color" Colors.pinHighlight
                                , Views.style "border-radius" "50%"
                                ]

                            else
                                []
                           )
                    )
                    []
                  <|
                    let
                        numPinnedResources =
                            List.length <| getPinnedResources model
                    in
                    [ if numPinnedResources > 0 then
                        Views.div
                            Views.Unidentified
                            Styles.pinIcon
                            [ onMouseEnter <| Hover <| Just PinIcon
                            , onMouseLeave <| Hover Nothing
                            ]
                            (Views.div
                                (Views.Id "pin-badge")
                                Styles.pinBadge
                                []
                                [ Views.div Views.Unidentified
                                    []
                                    []
                                    [ Views.text <|
                                        String.fromInt numPinnedResources
                                    ]
                                ]
                                :: viewPinMenuDropdown
                                    { pinnedResources = getPinnedResources model
                                    , pipeline = model.pipelineLocator
                                    , isPinMenuExpanded =
                                        model.hovered == Just PinIcon
                                    }
                            )

                      else
                        Views.div Views.Unidentified Styles.pinIcon [] []
                    ]
                , Views.div
                    (Views.Id "top-bar-pause-toggle")
                    (Styles.pauseToggle <| isPaused model.pipeline)
                    []
                    [ PauseToggle.view "17px"
                        userState
                        { pipeline = model.pipelineLocator
                        , isPaused = isPaused model.pipeline
                        , isToggleHovered =
                            model.hovered
                                == (Just <|
                                        PipelineButton model.pipelineLocator
                                   )
                        , isToggleLoading = model.isToggleLoading
                        }
                    ]
                ]
            ]
        ]


topBarOther : UserState -> Model -> Views.View Message
topBarOther userState model =
    Views.div
        (Views.Id "top-bar-app")
        ((Views.Styles.topBar <| isPaused model.pipeline)
            ++ [ Views.style "flex-direction" "row" ]
        )
        []
        [ TopBar.concourseLogo
        , Views.div
            (Views.Id "breadcrumbs")
            (Views.Styles.breadcrumbContainer ++ [ Views.style "overflow" "hidden" ])
            []
            [ Views.a
                (Views.Id "breadcrumb-pipeline")
                (Views.style "align-items" "center"
                    :: Views.Styles.breadcrumbItem True
                )
                [ href <|
                    Routes.toString <|
                        Routes.Pipeline
                            { id = model.pipelineLocator
                            , groups = []
                            }
                ]
                (TopBar.breadcrumbComponent "pipeline"
                    model.pipelineLocator.pipelineName
                )
            ]
        , viewPinMenu
            { pinnedResources = getPinnedResources model
            , pipeline = model.pipelineLocator
            , isPinMenuExpanded =
                model.hovered == Just PinIcon
            }
        , Views.div
            (Views.Id "top-bar-pause-toggle")
            (Styles.pauseToggle <| isPaused model.pipeline)
            []
            [ PauseToggle.view "17px"
                userState
                { pipeline = model.pipelineLocator
                , isPaused = isPaused model.pipeline
                , isToggleHovered =
                    model.hovered
                        == (Just <|
                                PipelineButton model.pipelineLocator
                           )
                , isToggleLoading = model.isToggleLoading
                }
            ]
        , Login.view userState model <| isPaused model.pipeline
        ]


route : Model -> Routes.Route
route model =
    Routes.Pipeline
        { id = model.pipelineLocator
        , groups = model.selectedGroups
        }


viewPinMenu :
    { pinnedResources : List ( String, Concourse.Version )
    , pipeline : Concourse.PipelineIdentifier
    , isPinMenuExpanded : Bool
    }
    -> Views.View Message
viewPinMenu ({ pinnedResources, isPinMenuExpanded } as params) =
    Views.div
        (Views.Id "pin-icon")
        (Styles.pinIconContainer isPinMenuExpanded)
        []
        [ if List.length pinnedResources > 0 then
            Views.div
                Views.Unidentified
                Styles.pinIcon
                [ onMouseEnter <| Hover <| Just PinIcon
                , onMouseLeave <| Hover Nothing
                ]
                (Views.div
                    (Views.Id "pin-badge")
                    Styles.pinBadge
                    []
                    [ Views.div
                        Views.Unidentified
                        []
                        []
                        [ Views.text <|
                            String.fromInt <|
                                List.length pinnedResources
                        ]
                    ]
                    :: viewPinMenuDropdown params
                )

          else
            Views.div Views.Unidentified Styles.pinIcon [] []
        ]


viewPinMenuDropdown :
    { pinnedResources : List ( String, Concourse.Version )
    , pipeline : Concourse.PipelineIdentifier
    , isPinMenuExpanded : Bool
    }
    -> List (Views.View Message)
viewPinMenuDropdown { pinnedResources, pipeline, isPinMenuExpanded } =
    if isPinMenuExpanded then
        [ Views.ul
            Views.Unidentified
            Styles.pinIconDropdown
            []
            (pinnedResources
                |> List.map
                    (\( resourceName, pinnedVersion ) ->
                        Views.li
                            Views.Unidentified
                            Styles.pinDropdownCursor
                            [ onClick
                                (GoToRoute <|
                                    Routes.Resource
                                        { id =
                                            { teamName = pipeline.teamName
                                            , pipelineName = pipeline.pipelineName
                                            , resourceName = resourceName
                                            }
                                        , page = Nothing
                                        }
                                )
                            ]
                            [ Views.div
                                Views.Unidentified
                                Styles.pinText
                                []
                                [ Views.text resourceName ]
                            , Views.table
                                Views.Unidentified
                                []
                                []
                                (pinnedVersion
                                    |> Dict.toList
                                    |> List.map
                                        (\( k, v ) ->
                                            Views.tr
                                                Views.Unidentified
                                                []
                                                []
                                                [ Views.td Views.Unidentified [] [] [ Views.text k ]
                                                , Views.td Views.Unidentified [] [] [ Views.text v ]
                                                ]
                                        )
                                )
                            ]
                    )
            )
        , Views.div Views.Unidentified Styles.pinHoverHighlight [] []
        ]

    else
        []


isPaused : WebData Concourse.Pipeline -> Bool
isPaused p =
    RemoteData.withDefault False (RemoteData.map .paused p)


viewSubPage : Model -> Views.View Message
viewSubPage model =
    Views.div Views.Unidentified
        []
        [ class "pipeline-view" ]
        [ viewGroupsBar model
        , Views.div Views.Unidentified
            []
            [ class "pipeline-content" ]
            [ Views.svg Views.Unidentified
                []
                [ SvgAttributes.class "pipeline-graph test" ]
                []
            , Views.div Views.Unidentified
                []
                [ if model.experiencingTurbulence then
                    class "error-message"

                  else
                    class "error-message hidden"
                ]
                [ Views.div Views.Unidentified
                    []
                    [ class "message" ]
                    [ Views.img Views.Unidentified [] [ src model.turbulenceImgSrc, class "seatbelt" ] []
                    , Views.p Views.Unidentified [] [] [ Views.text "experiencing turbulence" ]
                    , Views.p Views.Unidentified [] [ class "explanation" ] []
                    ]
                ]
            , if model.hideLegend then
                Views.text ""

              else
                Views.dl (Views.Id "legend")
                    []
                    [ class "legend" ]
                    [ Views.dt Views.Unidentified [] [ class "succeeded" ] []
                    , Views.dd Views.Unidentified [] [] [ Views.text "succeeded" ]
                    , Views.dt Views.Unidentified [] [ class "errored" ] []
                    , Views.dd Views.Unidentified [] [] [ Views.text "errored" ]
                    , Views.dt Views.Unidentified [] [ class "aborted" ] []
                    , Views.dd Views.Unidentified [] [] [ Views.text "aborted" ]
                    , Views.dt Views.Unidentified [] [ class "paused" ] []
                    , Views.dd Views.Unidentified [] [] [ Views.text "paused" ]
                    , Views.dt Views.Unidentified
                        [ Views.style "background-color" Colors.pinned ]
                        []
                        []
                    , Views.dd Views.Unidentified [] [] [ Views.text "pinned" ]
                    , Views.dt Views.Unidentified [] [ class "failed" ] []
                    , Views.dd Views.Unidentified [] [] [ Views.text "failed" ]
                    , Views.dt Views.Unidentified [] [ class "pending" ] []
                    , Views.dd Views.Unidentified [] [] [ Views.text "pending" ]
                    , Views.dt Views.Unidentified [] [ class "started" ] []
                    , Views.dd Views.Unidentified [] [] [ Views.text "started" ]
                    , Views.dt Views.Unidentified [] [ class "dotted" ] [ Views.text "." ]
                    , Views.dd Views.Unidentified [] [] [ Views.text "dependency" ]
                    , Views.dt Views.Unidentified [] [ class "solid" ] [ Views.text "-" ]
                    , Views.dd Views.Unidentified [] [] [ Views.text "dependency (trigger)" ]
                    ]
            , Views.table Views.Unidentified
                []
                [ class "lower-right-info" ]
                [ Views.tr Views.Unidentified
                    []
                    []
                    [ Views.td Views.Unidentified [] [ class "label" ] [ Views.text "cli:" ]
                    , Views.td Views.Unidentified
                        []
                        []
                        [ Views.ul Views.Unidentified [] [ class "cli-downloads" ] <|
                            List.map
                                (\cli ->
                                    Views.li Views.Unidentified
                                        []
                                        []
                                        [ Views.a
                                            Views.Unidentified
                                            (Styles.cliIcon cli)
                                            [ href <| Cli.downloadUrl cli
                                            , ariaLabel <| Cli.label cli
                                            , download ""
                                            ]
                                            []
                                        ]
                                )
                                Cli.clis
                        ]
                    ]
                , Views.tr Views.Unidentified
                    []
                    []
                    [ Views.td Views.Unidentified [] [ class "label" ] [ Views.text "version:" ]
                    , Views.td Views.Unidentified
                        []
                        []
                        [ Views.div (Views.Id "concourse-version")
                            []
                            []
                            [ Views.text "v"
                            , Views.span Views.Unidentified
                                []
                                [ class "number" ]
                                [ Views.text model.concourseVersion ]
                            ]
                        ]
                    ]
                ]
            ]
        ]


viewGroupsBar : Model -> Views.View Message
viewGroupsBar model =
    let
        groupList =
            case model.pipeline of
                RemoteData.Success pipeline ->
                    List.indexedMap
                        (viewGroup
                            { selectedGroups = selectedGroupsOrDefault model
                            , pipelineLocator = model.pipelineLocator
                            , hovered = model.hovered
                            }
                        )
                        pipeline.groups

                _ ->
                    []
    in
    if List.isEmpty groupList then
        Views.text ""

    else
        Views.div
            (Views.Id "groups-bar")
            Styles.groupsBar
            []
            groupList


viewGroup :
    { a
        | selectedGroups : List String
        , pipelineLocator : Concourse.PipelineIdentifier
        , hovered : Maybe Hoverable
    }
    -> Int
    -> Concourse.PipelineGroup
    -> Views.View Message
viewGroup { selectedGroups, pipelineLocator, hovered } idx grp =
    let
        url =
            Routes.toString <|
                Routes.Pipeline { id = pipelineLocator, groups = [ grp.name ] }
    in
    Views.a Views.Unidentified
        []
        ([ Html.Attributes.href <| url
         , onLeftClickOrShiftLeftClick
            (SetGroups [ grp.name ])
            (ToggleGroup grp)
         , onMouseEnter <| Hover <| Just <| JobGroup idx
         , onMouseLeave <| Hover Nothing
         ]
            ++ Styles.groupItem
                { selected = List.member grp.name selectedGroups
                , hovered = hovered == (Just <| JobGroup idx)
                }
        )
        [ Views.text grp.name ]


jobAppearsInGroups : List String -> Concourse.PipelineIdentifier -> Json.Encode.Value -> Bool
jobAppearsInGroups groupNames pi jobJson =
    let
        concourseJob =
            Json.Decode.decodeValue (Concourse.decodeJob pi) jobJson
    in
    case concourseJob of
        Ok cj ->
            anyIntersect cj.groups groupNames

        Err _ ->
            -- failed to check if job is in group
            False


expandJsonList : Json.Encode.Value -> List Json.Decode.Value
expandJsonList flatList =
    let
        result =
            Json.Decode.decodeValue (Json.Decode.list Json.Decode.value) flatList
    in
    case result of
        Ok res ->
            res

        Err _ ->
            []


filterJobs : Model -> Json.Encode.Value -> Json.Encode.Value
filterJobs model value =
    Json.Encode.list identity <|
        List.filter
            (jobAppearsInGroups (activeGroups model) model.pipelineLocator)
            (expandJsonList value)


activeGroups : Model -> List String
activeGroups model =
    case ( model.selectedGroups, model.pipeline |> RemoteData.toMaybe |> Maybe.andThen (List.head << .groups) ) of
        ( [], Just firstGroup ) ->
            [ firstGroup.name ]

        ( groups, _ ) ->
            groups


renderIfNeeded : ET Model
renderIfNeeded ( model, effects ) =
    case ( model.fetchedResources, model.fetchedJobs ) of
        ( Just fetchedResources, Just fetchedJobs ) ->
            let
                filteredFetchedJobs =
                    if List.isEmpty (activeGroups model) then
                        fetchedJobs

                    else
                        filterJobs model fetchedJobs
            in
            case ( model.renderedResources, model.renderedJobs ) of
                ( Just renderedResources, Just renderedJobs ) ->
                    if
                        (expandJsonList renderedJobs /= expandJsonList filteredFetchedJobs)
                            || (expandJsonList renderedResources /= expandJsonList fetchedResources)
                    then
                        ( { model
                            | renderedJobs = Just filteredFetchedJobs
                            , renderedResources = Just fetchedResources
                          }
                        , effects ++ [ RenderPipeline filteredFetchedJobs fetchedResources ]
                        )

                    else
                        ( model, effects )

                _ ->
                    ( { model
                        | renderedJobs = Just filteredFetchedJobs
                        , renderedResources = Just fetchedResources
                      }
                    , effects ++ [ RenderPipeline filteredFetchedJobs fetchedResources ]
                    )

        _ ->
            ( model, effects )


anyIntersect : List a -> List a -> Bool
anyIntersect list1 list2 =
    case list1 of
        [] ->
            False

        first :: rest ->
            if List.member first list2 then
                True

            else
                anyIntersect rest list2


toggleGroup : Concourse.PipelineGroup -> List String -> WebData Concourse.Pipeline -> List String
toggleGroup grp names mpipeline =
    if List.member grp.name names then
        List.filter ((/=) grp.name) names

    else if List.isEmpty names then
        grp.name :: getDefaultSelectedGroups mpipeline

    else
        grp.name :: names


selectedGroupsOrDefault : Model -> List String
selectedGroupsOrDefault model =
    if List.isEmpty model.selectedGroups then
        getDefaultSelectedGroups model.pipeline

    else
        model.selectedGroups


getDefaultSelectedGroups : WebData Concourse.Pipeline -> List String
getDefaultSelectedGroups pipeline =
    case pipeline of
        RemoteData.Success p ->
            case List.head p.groups of
                Nothing ->
                    []

                Just first ->
                    [ first.name ]

        _ ->
            []


getNextUrl : List String -> Model -> String
getNextUrl newGroups model =
    Routes.toString <|
        Routes.Pipeline { id = model.pipelineLocator, groups = newGroups }
