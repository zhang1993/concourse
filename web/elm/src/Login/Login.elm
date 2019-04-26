module Login.Login exposing (Model, myView, update, view, viewLoginState, myViewLoginState)

import Concourse
import EffectTransformer exposing (ET)
import Html exposing (Html)
import Html.Attributes exposing (attribute, href, id)
import Html.Events exposing (onClick)
import Login.Styles as Styles
import Views.Views as Views exposing (View)
import Message.Effects exposing (Effect(..))
import Message.Message exposing (Message(..))
import UserState exposing (UserState(..))


type alias Model r =
    { r | isUserMenuExpanded : Bool }


update : Message -> ET (Model r)
update msg ( model, effects ) =
    case msg of
        LogIn ->
            ( model, effects ++ [ RedirectToLogin ] )

        LogOut ->
            ( model, effects ++ [ SendLogOutRequest ] )

        ToggleUserMenu ->
            ( { model | isUserMenuExpanded = not model.isUserMenuExpanded }
            , effects
            )

        _ ->
            ( model, effects )


view :
    UserState
    -> Model r
    -> Bool
    -> Html Message
view userState model isPaused =
    myView userState model isPaused |> Views.toHtml


myView :
    UserState
    -> Model r
    -> Bool
    -> View Message
myView userState model isPaused =
    Views.div
        (Views.Id "login-component")
        Styles.loginComponent
        []
        (myViewLoginState userState model.isUserMenuExpanded isPaused)


viewLoginState : UserState -> Bool -> Bool -> List (Html Message)
viewLoginState userState isUserMenuExpanded isPaused =
    myViewLoginState userState isUserMenuExpanded isPaused |> List.map Views.toHtml

myViewLoginState : UserState -> Bool -> Bool -> List (Views.View Message)
myViewLoginState userState isUserMenuExpanded isPaused =
    case userState of
        UserStateUnknown ->
            []

        UserStateLoggedOut ->
            [ Views.div
                (Views.Id "login-container")
                (Styles.loginContainer isPaused)
                [ attribute "aria-label" "Log In"
                , onClick LogIn
                ]
                [ Views.div
                    (Views.Id "login-item")
                    Styles.loginItem
                    []
                    [ Views.a
                        Views.Unidentified
                        []
                        [ href "/sky/login" ]
                        [ Views.text "login" ]
                    ]
                ]
            ]

        UserStateLoggedIn user ->
            [ Views.div
                (Views.Id "login-container")
                (Styles.loginContainer isPaused)
                [ onClick ToggleUserMenu ]
                [ Views.div
                    (Views.Id "user-id")
                    Styles.loginItem
                    []
                    (Views.div
                        Views.Unidentified
                        Styles.loginText
                        []
                        [ Views.text (userDisplayName user) ]
                        :: (if isUserMenuExpanded then
                                [ Views.div
                                    (Views.Id "logout-button")
                                    Styles.logoutButton
                                    [ onClick LogOut ]
                                    [ Views.text "logout" ]
                                ]

                            else
                                []
                           )
                    )
                ]
            ]


userDisplayName : Concourse.User -> String
userDisplayName user =
    Maybe.withDefault user.id <|
        List.head <|
            List.filter
                (not << String.isEmpty)
                [ user.userName, user.name, user.email ]
