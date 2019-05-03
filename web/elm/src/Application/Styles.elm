module Application.Styles exposing (disableInteraction)

import Views.Views as Views exposing (style)


disableInteraction : List Views.Style
disableInteraction =
    [ style "cursor" "default"
    , style "user-select" "none"
    , style "-ms-user-select" "none"
    , style "-moz-user-select" "none"
    , style "-khtml-user-select" "none"
    , style "-webkit-user-select" "none"
    , style "-webkit-touch-callout" "none"
    ]
