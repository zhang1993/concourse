module Login.Styles exposing
    ( loginComponent
    , loginContainer
    , loginItem
    , loginText
    , logoutButton
    )

import Colors
import Html
import Html.Attributes exposing (style)
import Login.Views as Views


loginComponent : List Views.Style
loginComponent =
    [ Views.style "max-width" "20%" ]


loginContainer : Bool -> List Views.Style
loginContainer isPaused =
    [ Views.style "position" "relative"
    , Views.style "display" "flex"
    , Views.style "flex-direction" "column"
    , Views.style "border-left" <|
        "1px solid "
            ++ (if isPaused then
                    Colors.pausedTopbarSeparator

                else
                    Colors.background
               )
    , Views.style "line-height" "54px"
    ]


loginItem : List Views.Style
loginItem =
    [ Views.style "padding" "0 30px"
    , Views.style "cursor" "pointer"
    , Views.style "display" "flex"
    , Views.style "align-items" "center"
    , Views.style "justify-content" "center"
    , Views.style "flex-grow" "1"
    ]


loginText : List Views.Style
loginText =
    [ Views.style "overflow" "hidden"
    , Views.style "text-overflow" "ellipsis"
    ]


logoutButton : List Views.Style
logoutButton =
    [ Views.style "position" "absolute"
    , Views.style "top" "55px"
    , Views.style "background-color" Colors.frame
    , Views.style "height" "54px"
    , Views.style "width" "100%"
    , Views.style "border-top" <| "1px solid " ++ Colors.background
    , Views.style "cursor" "pointer"
    , Views.style "display" "flex"
    , Views.style "align-items" "center"
    , Views.style "justify-content" "center"
    , Views.style "flex-grow" "1"
    ]
