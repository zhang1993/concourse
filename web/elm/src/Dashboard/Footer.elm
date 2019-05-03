module Dashboard.Footer exposing (handleDelivery, view)

import Concourse.Cli as Cli
import Concourse.PipelineStatus as PipelineStatus exposing (PipelineStatus(..))
import Dashboard.Group.Models exposing (Group)
import Dashboard.Models exposing (Dropdown(..), FooterModel)
import Dashboard.Styles as Styles
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, download, href, id, style)
import Html.Events exposing (onMouseEnter, onMouseLeave)
import Keyboard
import Message.Effects as Effects
import Message.Message exposing (Hoverable(..), Message(..))
import Message.Subscription exposing (Delivery(..), Interval(..))
import Routes
import ScreenSize
import Views.Icon as Icon
import Views.Views as Views


handleDelivery :
    Delivery
    -> ( FooterModel r, List Effects.Effect )
    -> ( FooterModel r, List Effects.Effect )
handleDelivery delivery ( model, effects ) =
    case delivery of
        KeyDown keyEvent ->
            case keyEvent.code of
                -- '/' key
                Keyboard.Slash ->
                    if keyEvent.shiftKey && model.dropdown == Hidden then
                        ( { model
                            | showHelp =
                                if
                                    model.groups
                                        |> List.concatMap .pipelines
                                        |> List.isEmpty
                                then
                                    False

                                else
                                    not model.showHelp
                          }
                        , effects
                        )

                    else
                        ( model, effects )

                _ ->
                    ( { model | hideFooter = False, hideFooterCounter = 0 }
                    , effects
                    )

        Moused ->
            ( { model | hideFooter = False, hideFooterCounter = 0 }, effects )

        ClockTicked OneSecond _ ->
            ( if model.hideFooterCounter > 4 then
                { model | hideFooter = True }

              else
                { model | hideFooterCounter = model.hideFooterCounter + 1 }
            , effects
            )

        _ ->
            ( model, effects )


view : FooterModel r -> Views.View Message
view model =
    if model.showHelp then
        keyboardHelp

    else if not model.hideFooter then
        infoBar model

    else
        Views.text ""


keyboardHelp : Views.View Message
keyboardHelp =
    Views.div (Views.Id "keyboard-help")
        []
        [ class "keyboard-help" ]
        [ Views.div Views.Unidentified
            []
            [ class "help-title" ]
            [ Views.text "keyboard shortcuts" ]
        , Views.div Views.Unidentified
            []
            [ class "help-line" ]
            [ Views.div Views.Unidentified
                []
                [ class "keys" ]
                [ Views.span Views.Unidentified
                    []
                    [ class "key" ]
                    [ Views.text "/" ]
                ]
            , Views.text "search"
            ]
        , Views.div Views.Unidentified
            []
            [ class "help-line" ]
            [ Views.div Views.Unidentified
                []
                [ class "keys" ]
                [ Views.span Views.Unidentified
                    []
                    [ class "key" ]
                    [ Views.text "?" ]
                ]
            , Views.text "hide/show help"
            ]
        ]


infoBar :
    { a
        | hovered : Maybe Hoverable
        , screenSize : ScreenSize.ScreenSize
        , version : String
        , highDensity : Bool
        , groups : List Group
    }
    -> Views.View Message
infoBar model =
    Views.div
        (Views.Id "dashboard-info")
        (Styles.infoBar
            { hideLegend = hideLegend model
            , screenSize = model.screenSize
            }
        )
        []
        [ legend model
        , concourseInfo model
        ]


legend :
    { a
        | groups : List Group
        , screenSize : ScreenSize.ScreenSize
        , highDensity : Bool
    }
    -> Views.View Message
legend model =
    if hideLegend model then
        Views.text ""

    else
        Views.div (Views.Id "legend") Styles.legend [] <|
            List.map legendItem
                [ PipelineStatusPending False
                , PipelineStatusPaused
                ]
                ++ Views.div Views.Unidentified
                    Styles.legendItem
                    []
                    [ Icon.icon
                        { sizePx = 20
                        , image = "ic-running-legend.svg"
                        }
                        []
                    , Views.div Views.Unidentified [ Views.style "width" "10px" ] [] []
                    , Views.text "running"
                    ]
                :: List.map legendItem
                    [ PipelineStatusFailed PipelineStatus.Running
                    , PipelineStatusErrored PipelineStatus.Running
                    , PipelineStatusAborted PipelineStatus.Running
                    , PipelineStatusSucceeded PipelineStatus.Running
                    ]
                ++ [ legendSeparator model.screenSize ]
                ++ [ toggleView model.highDensity ]


concourseInfo :
    { a | version : String, hovered : Maybe Hoverable }
    -> Views.View Message
concourseInfo { version, hovered } =
    Views.div (Views.Id "concourse-info")
        Styles.info
        []
        [ Views.div Views.Unidentified
            Styles.infoItem
            []
            [ Views.text <| "version: v" ++ version ]
        , Views.div Views.Unidentified Styles.infoItem [] <|
            [ Views.span Views.Unidentified
                [ Views.style "margin-right" "10px" ]
                []
                [ Views.text "cli: " ]
            ]
                ++ List.map (cliIcon hovered) Cli.clis
        ]


hideLegend : { a | groups : List Group } -> Bool
hideLegend { groups } =
    List.isEmpty (groups |> List.concatMap .pipelines)


legendItem : PipelineStatus -> Views.View Message
legendItem status =
    Views.div Views.Unidentified
        Styles.legendItem
        []
        [ PipelineStatus.icon status
        , Views.div Views.Unidentified [ Views.style "width" "10px" ] [] []
        , Views.text <| PipelineStatus.show status
        ]


toggleView : Bool -> Views.View Message
toggleView highDensity =
    Views.a Views.Unidentified
        Styles.highDensityToggle
        [ href <| Routes.toString <| Routes.dashboardRoute (not highDensity)
        , attribute "aria-label" "Toggle high-density view"
        ]
        [ Views.div Views.Unidentified (Styles.highDensityIcon highDensity) [] []
        , Views.text "high-density"
        ]


legendSeparator : ScreenSize.ScreenSize -> Views.View Message
legendSeparator screenSize =
    case screenSize of
        ScreenSize.Phone ->
            Views.text ""

        ScreenSize.Tablet ->
            Views.text ""

        ScreenSize.Desktop ->
            Views.div Views.Unidentified Styles.legendSeparator [] [ Views.text "|" ]

        ScreenSize.BigDesktop ->
            Views.div Views.Unidentified Styles.legendSeparator [] [ Views.text "|" ]


cliIcon : Maybe Hoverable -> Cli.Cli -> Views.View Message
cliIcon hovered cli =
    Views.a
        (Views.Id <| "cli-" ++ Cli.id cli)
        (Styles.infoCliIcon
            { hovered = hovered == (Just <| FooterCliIcon cli)
            , cli = cli
            }
        )
        [ href <| Cli.downloadUrl cli
        , attribute "aria-label" <| Cli.label cli
        , onMouseEnter <| Hover <| Just <| FooterCliIcon cli
        , onMouseLeave <| Hover Nothing
        , download ""
        ]
        []
